package batcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Layr-Labs/eigenda/common"
	"github.com/Layr-Labs/eigenda/core"
	"github.com/Layr-Labs/eigenda/disperser"
	"github.com/gammazero/workerpool"
	"github.com/wealdtech/go-merkletree"
)

const encodingInterval = 2 * time.Second

var errNoEncodedResults = errors.New("no encoded results")

type EncodedSizeNotifier struct {
	mu sync.Mutex

	Notify chan struct{}
	// threshold is the size of the total encoded blob results in bytes that triggers the notifier
	threshold uint
	// active is set to false after the notifier is triggered to prevent it from triggering again for the same batch
	// This is reset when CreateBatch is called and the encoded results have been consumed
	active bool
}

type StreamerConfig struct {

	// SRSOrder is the order of the SRS used for encoding
	SRSOrder int
	// EncodingRequestTimeout is the timeout for each encoding request
	EncodingRequestTimeout time.Duration

	// EncodingQueueLimit is the maximum number of encoding requests that can be queued
	EncodingQueueLimit int

	// PoolSize is the number of workers in the worker pool
	PoolSize int
}

type EncodingStreamer struct {
	StreamerConfig

	mu sync.RWMutex

	EncodedBlobstore     *encodedBlobStore
	ReferenceBlockNumber uint
	Pool                 *workerpool.WorkerPool
	EncodedSizeNotifier  *EncodedSizeNotifier

	blobStore             disperser.BlobStore
	chainState            core.IndexedChainState
	encoderClient         disperser.EncoderClient
	assignmentCoordinator core.AssignmentCoordinator

	encodingCtxCancelFuncs []context.CancelFunc

	logger common.Logger
}

type batchMetadata struct {
	QuorumInfos map[core.QuorumID]QuorumInfo
	State       *core.IndexedOperatorState
}

type batch struct {
	EncodedBlobs  []core.EncodedBlob
	BlobMetadata  []*disperser.BlobMetadata
	BlobHeaders   []*core.BlobHeader
	BatchHeader   *core.BatchHeader
	BatchMetadata *batchMetadata
	MerkleTree    *merkletree.MerkleTree
}

func NewEncodedSizeNotifier(notify chan struct{}, threshold uint) *EncodedSizeNotifier {
	return &EncodedSizeNotifier{
		Notify:    notify,
		threshold: threshold,
		active:    true,
	}
}

func NewEncodingStreamer(
	config StreamerConfig,
	blobStore disperser.BlobStore,
	chainState core.IndexedChainState,
	encoderClient disperser.EncoderClient,
	assignmentCoordinator core.AssignmentCoordinator,
	encodedSizeNotifier *EncodedSizeNotifier,
	logger common.Logger) (*EncodingStreamer, error) {
	if config.EncodingQueueLimit <= 0 {
		return nil, fmt.Errorf("EncodingQueueLimit should be greater than 0")
	}
	return &EncodingStreamer{
		StreamerConfig:         config,
		EncodedBlobstore:       newEncodedBlobStore(logger),
		ReferenceBlockNumber:   uint(0),
		Pool:                   workerpool.New(config.PoolSize),
		EncodedSizeNotifier:    encodedSizeNotifier,
		blobStore:              blobStore,
		chainState:             chainState,
		encoderClient:          encoderClient,
		assignmentCoordinator:  assignmentCoordinator,
		encodingCtxCancelFuncs: make([]context.CancelFunc, 0),
		logger:                 logger,
	}, nil
}

func (e *EncodingStreamer) Start(ctx context.Context) error {
	encoderChan := make(chan EncodingResultOrStatus)

	// goroutine for handling blob encoding responses
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case response := <-encoderChan:
				err := e.ProcessEncodedBlobs(ctx, response)
				if err != nil {
					if !errors.Is(err, context.Canceled) {
						e.logger.Error("error processing encoded blobs", "err", err)
					}
				}
			}
		}
	}()

	// goroutine for making blob encoding requests
	go func() {
		ticker := time.NewTicker(encodingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := e.RequestEncoding(ctx, encoderChan)
				if err != nil {
					e.logger.Error("error requesting encoding", "err", err)
				}
			}
		}
	}()

	return nil
}

func (e *EncodingStreamer) dedupRequests(metadatas []*disperser.BlobMetadata, referenceBlockNumber uint) []*disperser.BlobMetadata {
	res := make([]*disperser.BlobMetadata, 0)
	for _, meta := range metadatas {
		allQuorumsRequested := true
		// check if the blob has been requested for all quorums
		for _, quorum := range meta.RequestMetadata.SecurityParams {
			if !e.EncodedBlobstore.HasEncodingRequested(meta.GetBlobKey(), quorum.QuorumID, referenceBlockNumber) {
				allQuorumsRequested = false
				break
			}
		}
		if !allQuorumsRequested {
			res = append(res, meta)
		}
	}

	return res
}

func (e *EncodingStreamer) RequestEncoding(ctx context.Context, encoderChan chan EncodingResultOrStatus) error {
	stageTimer := time.Now()
	// pull new blobs and send to encoder
	metadatas, err := e.blobStore.GetBlobMetadataByStatus(ctx, disperser.Processing)
	if err != nil {
		return fmt.Errorf("error getting blob metadatas: %w", err)
	}
	if len(metadatas) == 0 {
		e.logger.Info("no new metadatas to encode")
		return nil
	}

	// read lock to access e.ReferenceBlockNumber
	e.mu.RLock()
	referenceBlockNumber := e.ReferenceBlockNumber
	e.mu.RUnlock()

	if referenceBlockNumber == 0 {
		// Update the reference block number for the next iteration
		blockNumber, err := e.chainState.GetCurrentBlockNumber()
		if err != nil {
			return fmt.Errorf("failed to get current block number, won't request encoding: %w", err)
		} else {
			e.mu.Lock()
			e.ReferenceBlockNumber = blockNumber
			e.mu.Unlock()
			referenceBlockNumber = blockNumber
		}
	}

	e.logger.Trace("[encodingstreamer] metadata in processing status", "numMetadata", len(metadatas))
	metadatas = e.dedupRequests(metadatas, referenceBlockNumber)
	if len(metadatas) == 0 {
		e.logger.Info("no new metadatas to encode")
		return nil
	}

	waitingQueueSize := e.Pool.WaitingQueueSize()
	numMetadatastoProcess := e.EncodingQueueLimit - waitingQueueSize
	if numMetadatastoProcess > len(metadatas) {
		numMetadatastoProcess = len(metadatas)
	}
	if numMetadatastoProcess <= 0 {
		// encoding queue is full
		e.logger.Warn("[RequestEncoding] worker pool queue is full. skipping this round of encoding requests", "waitingQueueSize", waitingQueueSize, "encodingQueueLimit", e.EncodingQueueLimit)
		return nil
	}
	// only process subset of blobs so it doesn't exceed the EncodingQueueLimit
	// TODO: this should be done at the request time and keep the cursor so that we don't fetch the same metadata every time
	metadatas = metadatas[:numMetadatastoProcess]

	e.logger.Trace("[encodingstreamer] new metadatas to encode", "numMetadata", len(metadatas), "duration", time.Since(stageTimer))

	batchMetadata, err := e.getBatchMetadata(ctx, metadatas, referenceBlockNumber)
	if err != nil {
		return fmt.Errorf("error getting quorum infos: %w", err)
	}

	metadataByKey := make(map[disperser.BlobKey]*disperser.BlobMetadata, 0)
	for _, metadata := range metadatas {
		metadataByKey[metadata.GetBlobKey()] = metadata
	}

	stageTimer = time.Now()
	blobs, err := e.blobStore.GetBlobsByMetadata(ctx, metadatas)
	if err != nil {
		return fmt.Errorf("error getting blobs from blob store: %w", err)
	}
	e.logger.Trace("[RequestEncoding] retrieved blobs to encode", "numBlobs", len(blobs), "duration", time.Since(stageTimer))

	e.logger.Trace("[RequestEncoding] encoding blobs...", "numBlobs", len(blobs), "blockNumber", referenceBlockNumber)

	for i := range metadatas {
		metadata := metadatas[i]

		e.RequestEncodingForBlob(ctx, metadata, blobs[metadata.GetBlobKey()], batchMetadata, referenceBlockNumber, encoderChan)
	}

	return nil
}

type pendingRequestInfo struct {
	BlobQuorumInfo *core.BlobQuorumInfo
	EncodingParams core.EncodingParams
}

func (e *EncodingStreamer) RequestEncodingForBlob(ctx context.Context, metadata *disperser.BlobMetadata, blob *core.Blob, batchMetadata *batchMetadata, referenceBlockNumber uint, encoderChan chan EncodingResultOrStatus) {

	// Validate the encoding parameters for each quorum

	blobKey := metadata.GetBlobKey()

	pending := make([]pendingRequestInfo, 0, len(metadata.RequestMetadata.SecurityParams))

	for ind := range metadata.RequestMetadata.SecurityParams {

		quorum := metadata.RequestMetadata.SecurityParams[ind]
		// Check if the blob has already been encoded for this quorum
		if e.EncodedBlobstore.HasEncodingRequested(blobKey, quorum.QuorumID, referenceBlockNumber) {
			continue
		}

		quorumInfo := batchMetadata.QuorumInfos[quorum.QuorumID]
		blobLength := core.GetBlobLength(metadata.RequestMetadata.BlobSize)
		numOperators := uint(len(quorumInfo.Assignments))
		chunkLength, err := e.assignmentCoordinator.GetMinimumChunkLength(numOperators, blobLength, quorumInfo.QuantizationFactor, quorum.QuorumThreshold, quorum.AdversaryThreshold)
		if err != nil {
			// This error shouldn't happen because we check blob headers before adding them blob store
			e.logger.Error("[RequestEncodingForBlob] invalid request parameters", "err", err)
			continue
		}
		params, err := core.GetEncodingParams(chunkLength, quorumInfo.Info.TotalChunks)
		if err != nil {
			e.logger.Error("[RequestEncodingForBlob] error getting encoding params", "err", err)
			continue
		}

		err = core.ValidateEncodingParams(params, int(blobLength), e.SRSOrder)
		if err != nil {
			e.logger.Error("[RequestEncodingForBlob] invalid encoding params", "err", err)
			// Cancel the blob
			err := e.blobStore.MarkBlobFailed(ctx, blobKey)
			if err != nil {
				e.logger.Error("[RequestEncodingForBlob] error marking blob failed", "err", err)
			}
			return
		}

		blobQuorumInfo := &core.BlobQuorumInfo{
			SecurityParam: core.SecurityParam{
				QuorumID:           quorum.QuorumID,
				AdversaryThreshold: quorum.AdversaryThreshold,
				QuorumThreshold:    quorum.QuorumThreshold,
				QuorumRate:         quorum.QuorumRate,
			},
			QuantizationFactor: quorumInfo.QuantizationFactor,
			EncodedBlobLength:  params.ChunkLength * quorumInfo.QuantizationFactor * numOperators,
		}

		pending = append(pending, pendingRequestInfo{
			BlobQuorumInfo: blobQuorumInfo,
			EncodingParams: params,
		})
	}

	// Execute the encoding requests
	for ind := range pending {

		res := pending[ind]

		// Create a new context for each encoding request
		// This allows us to cancel all outstanding encoding requests when we create a new batch
		// This is necessary because an encoding request is dependent on the reference block number
		// If the reference block number changes, we need to cancel all outstanding encoding requests
		// and re-request them with the new reference block number
		encodingCtx, cancel := context.WithTimeout(ctx, e.EncodingRequestTimeout)
		e.mu.Lock()
		e.encodingCtxCancelFuncs = append(e.encodingCtxCancelFuncs, cancel)
		e.mu.Unlock()
		e.Pool.Submit(func() {
			defer cancel()
			commits, chunks, err := e.encoderClient.EncodeBlob(encodingCtx, blob.Data, res.EncodingParams)
			if err != nil {
				encoderChan <- EncodingResultOrStatus{Err: err, EncodingResult: EncodingResult{
					BlobMetadata:   metadata,
					BlobQuorumInfo: res.BlobQuorumInfo,
				}}
				return
			}

			encoderChan <- EncodingResultOrStatus{
				EncodingResult: EncodingResult{
					BlobMetadata:         metadata,
					ReferenceBlockNumber: referenceBlockNumber,
					BlobQuorumInfo:       res.BlobQuorumInfo,
					Commitment:           commits,
					Chunks:               chunks,
					Assignments:          batchMetadata.QuorumInfos[res.BlobQuorumInfo.QuorumID].Assignments,
				},
				Err: nil,
			}
		})
		e.EncodedBlobstore.PutEncodingRequest(blobKey, res.BlobQuorumInfo.QuorumID)

	}

}

func (e *EncodingStreamer) ProcessEncodedBlobs(ctx context.Context, result EncodingResultOrStatus) error {
	if result.Err != nil {
		e.EncodedBlobstore.DeleteEncodingRequest(result.BlobMetadata.GetBlobKey(), result.BlobQuorumInfo.QuorumID)
		return fmt.Errorf("error encoding blob: %w", result.Err)
	}

	err := e.EncodedBlobstore.PutEncodingResult(&result.EncodingResult)
	if err != nil {
		return fmt.Errorf("failed to putEncodedBlob: %w", err)
	}

	encodedSize := e.EncodedBlobstore.GetEncodedResultSize()
	if e.EncodedSizeNotifier.threshold > 0 && encodedSize >= e.EncodedSizeNotifier.threshold {
		e.EncodedSizeNotifier.mu.Lock()

		if e.EncodedSizeNotifier.active {
			e.logger.Info("encoded size threshold reached", "size", encodedSize)
			e.EncodedSizeNotifier.Notify <- struct{}{}
			// make sure this doesn't keep triggering before encoded blob store is reset
			e.EncodedSizeNotifier.active = false
		}
		e.EncodedSizeNotifier.mu.Unlock()
	}

	return nil
}

// CreateBatch makes a batch from all blobs in the encoded blob store.
// If successful, it returns a batch, and updates the reference block number for next batch to use.
// Otherwise, it returns an error and keeps the blobs in the encoded blob store.
// This function is meant to be called periodically in a single goroutine as it resets the state of the encoded blob store.
func (e *EncodingStreamer) CreateBatch() (*batch, error) {
	// lock to update e.ReferenceBlockNumber
	e.mu.Lock()
	defer e.mu.Unlock()
	// Cancel outstanding encoding requests
	// Assumption: `CreateBatch` will be called at an interval longer than time it takes to encode a single blob
	if len(e.encodingCtxCancelFuncs) > 0 {
		e.logger.Info("[CreateBatch] canceling outstanding encoding requests", "count", len(e.encodingCtxCancelFuncs))
		for _, cancel := range e.encodingCtxCancelFuncs {
			cancel()
		}
		e.encodingCtxCancelFuncs = make([]context.CancelFunc, 0)
	}

	// If there were no requested blobs between the last batch and now, there is no need to create a new batch
	if e.ReferenceBlockNumber == 0 {
		blockNumber, err := e.chainState.GetCurrentBlockNumber()
		if err != nil {
			e.logger.Error("[CreateBatch] failed to get current block number. will not clean up the encoded blob store.", "err", err)
		} else {
			_ = e.EncodedBlobstore.GetNewAndDeleteStaleEncodingResults(blockNumber)
		}
		return nil, errNoEncodedResults
	}

	// Delete any encoded results that are not from the current batching iteration (i.e. that has different reference block number)
	// If any pending encoded results are discarded here, it will be re-requested in the next iteration
	encodedResults := e.EncodedBlobstore.GetNewAndDeleteStaleEncodingResults(e.ReferenceBlockNumber)

	// Reset the notifier
	e.EncodedSizeNotifier.mu.Lock()
	e.EncodedSizeNotifier.active = true
	e.EncodedSizeNotifier.mu.Unlock()

	e.logger.Info("[CreateBatch] creating a batch...", "numBlobs", len(encodedResults), "refblockNumber", e.ReferenceBlockNumber)
	if len(encodedResults) == 0 {
		return nil, errNoEncodedResults
	}

	encodedBlobByKey := make(map[disperser.BlobKey]core.EncodedBlob)
	blobQuorums := make(map[disperser.BlobKey][]*core.BlobQuorumInfo)
	blobHeaderByKey := make(map[disperser.BlobKey]*core.BlobHeader)
	metadataByKey := make(map[disperser.BlobKey]*disperser.BlobMetadata)
	for i := range encodedResults {
		// each result represent an encoded result per (blob, quorum param)
		// if the same blob has been dispersed multiple time with different security params,
		// there will be multiple encoded results for that (blob, quorum)
		result := encodedResults[i]
		blobKey := result.BlobMetadata.GetBlobKey()
		if _, ok := encodedBlobByKey[blobKey]; !ok {
			metadataByKey[blobKey] = result.BlobMetadata
			blobQuorums[blobKey] = make([]*core.BlobQuorumInfo, 0)
			encodedBlobByKey[blobKey] = make(core.EncodedBlob)
		}

		// Populate the assigned bundles
		for opID, assignment := range result.Assignments {
			blobMessage, ok := encodedBlobByKey[blobKey][opID]
			if !ok {
				blobHeader := &core.BlobHeader{
					BlobCommitments: *result.Commitment,
				}
				blobHeaderByKey[blobKey] = blobHeader
				blobMessage = &core.BlobMessage{
					BlobHeader: blobHeader,
					Bundles:    make(core.Bundles),
				}
				encodedBlobByKey[blobKey][opID] = blobMessage
			}
			blobMessage.Bundles[result.BlobQuorumInfo.QuorumID] = append(blobMessage.Bundles[result.BlobQuorumInfo.QuorumID], result.Chunks[assignment.StartIndex:assignment.StartIndex+assignment.NumChunks]...)
		}

		blobQuorums[blobKey] = append(blobQuorums[blobKey], result.BlobQuorumInfo)
	}

	// Populate the blob quorum infos
	for blobKey, encodedBlob := range encodedBlobByKey {
		for _, blobMessage := range encodedBlob {
			blobMessage.BlobHeader.QuorumInfos = blobQuorums[blobKey]
		}
	}

	for blobKey, metadata := range metadataByKey {
		quorumPresent := make(map[core.QuorumID]bool)
		for _, quorum := range blobQuorums[blobKey] {
			quorumPresent[quorum.QuorumID] = true
		}
		for _, quorum := range metadata.RequestMetadata.SecurityParams {
			_, ok := quorumPresent[quorum.QuorumID]
			if !ok {
				// Delete the blobKey. These encoded blobs will be automatically removed by the next run of
				// RequestEncoding
				delete(metadataByKey, blobKey)
				break
			}
		}
	}

	// Transform maps to slices so orders in different slices match
	encodedBlobs := make([]core.EncodedBlob, len(metadataByKey))
	blobHeaders := make([]*core.BlobHeader, len(metadataByKey))
	metadatas := make([]*disperser.BlobMetadata, len(metadataByKey))
	i := 0
	for key := range metadataByKey {
		encodedBlobs[i] = encodedBlobByKey[key]
		blobHeaders[i] = blobHeaderByKey[key]
		metadatas[i] = metadataByKey[key]
		i++
	}

	batchMetadata, err := e.getBatchMetadata(context.Background(), metadatas, e.ReferenceBlockNumber)
	if err != nil {
		return nil, err
	}

	// Populate the batch header
	batchHeader := &core.BatchHeader{
		ReferenceBlockNumber: e.ReferenceBlockNumber,
		BatchRoot:            [32]byte{},
	}

	tree, err := batchHeader.SetBatchRoot(blobHeaders)
	if err != nil {
		return nil, err
	}

	e.ReferenceBlockNumber = 0

	return &batch{
		EncodedBlobs:  encodedBlobs,
		BatchHeader:   batchHeader,
		BlobHeaders:   blobHeaders,
		BlobMetadata:  metadatas,
		BatchMetadata: batchMetadata,
		MerkleTree:    tree,
	}, nil
}

func (e *EncodingStreamer) getBatchMetadata(ctx context.Context, metadatas []*disperser.BlobMetadata, blockNumber uint) (*batchMetadata, error) {
	quorums := make(map[core.QuorumID]QuorumInfo, 0)
	for _, metadata := range metadatas {
		for _, quorum := range metadata.RequestMetadata.SecurityParams {
			quorums[quorum.QuorumID] = QuorumInfo{}
		}
	}

	quorumIds := make([]core.QuorumID, len(quorums))
	i := 0
	for id := range quorums {
		quorumIds[i] = id
		i++
	}

	// Get the operator state
	state, err := e.chainState.GetIndexedOperatorState(ctx, blockNumber, quorumIds)
	if err != nil {
		return nil, fmt.Errorf("error getting operator state at block number %d: %w", blockNumber, err)
	}

	for quorumID := range quorums {
		assignments, info, err := e.assignmentCoordinator.GetAssignments(state.OperatorState, quorumID, QuantizationFactor)
		if err != nil {
			return nil, err
		}
		quorums[quorumID] = QuorumInfo{
			Assignments:        assignments,
			Info:               info,
			QuantizationFactor: QuantizationFactor,
		}
	}

	return &batchMetadata{
		QuorumInfos: quorums,
		State:       state,
	}, nil
}
