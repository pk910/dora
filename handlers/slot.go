package handlers

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/attestantio/go-eth2-client/spec"
	"github.com/attestantio/go-eth2-client/spec/bellatrix"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/gorilla/mux"
	"github.com/juliangruber/go-intersect"
	"github.com/sirupsen/logrus"

	"github.com/pk910/dora/db"
	"github.com/pk910/dora/dbtypes"
	"github.com/pk910/dora/rpc"
	"github.com/pk910/dora/services"
	"github.com/pk910/dora/templates"
	"github.com/pk910/dora/types"
	"github.com/pk910/dora/types/models"
	"github.com/pk910/dora/utils"
)

// Index will return the main "index" page using a go template
func Slot(w http.ResponseWriter, r *http.Request) {
	var slotTemplateFiles = append(layoutTemplateFiles,
		"slot/slot.html",
		"slot/overview.html",
		"slot/transactions.html",
		"slot/attestations.html",
		"slot/deposits.html",
		"slot/withdrawals.html",
		"slot/voluntary_exits.html",
		"slot/slashings.html",
		"slot/blobs.html",
		"slot/stateless.html",
	)
	var notfoundTemplateFiles = append(layoutTemplateFiles,
		"slot/notfound.html",
	)

	vars := mux.Vars(r)
	slotOrHash := strings.Replace(vars["slotOrHash"], "0x", "", -1)
	blockSlot := int64(-1)
	blockRootHash, err := hex.DecodeString(slotOrHash)
	if err != nil || len(slotOrHash) != 64 {
		blockRootHash = []byte{}
		blockSlot, err = strconv.ParseInt(vars["slotOrHash"], 10, 64)
		if err != nil || blockSlot >= 2147483648 { // block slot must be lower then max int4
			data := InitPageData(w, r, "blockchain", "/slots", fmt.Sprintf("Slot %v", slotOrHash), notfoundTemplateFiles)
			w.Header().Set("Content-Type", "text/html")
			if handleTemplateError(w, r, "slot.go", "Slot", "blockSlot", templates.GetTemplate(notfoundTemplateFiles...).ExecuteTemplate(w, "layout", data)) != nil {
				return // an error has occurred and was processed
			}
			return
		}
	}

	urlArgs := r.URL.Query()
	loadDuties := urlArgs.Has("duties")

	var pageData *models.SlotPageData
	var pageError error
	pageError = services.GlobalCallRateLimiter.CheckCallLimit(r, 1)
	if pageError == nil {
		pageData, pageError = getSlotPageData(blockSlot, blockRootHash, loadDuties)
	}
	if pageError != nil {
		handlePageError(w, r, pageError)
		return
	}
	if pageData == nil {
		data := InitPageData(w, r, "blockchain", "/slots", fmt.Sprintf("Slot %v", slotOrHash), notfoundTemplateFiles)
		data.Data = "slot"
		w.Header().Set("Content-Type", "text/html")
		if handleTemplateError(w, r, "slot.go", "Slot", "notFound", templates.GetTemplate(notfoundTemplateFiles...).ExecuteTemplate(w, "layout", data)) != nil {
			return // an error has occurred and was processed
		}
		return
	}

	if urlArgs.Has("blob") && pageData.Block != nil {
		commitment, err := hex.DecodeString(strings.Replace(urlArgs.Get("blob"), "0x", "", -1))
		var blobData *dbtypes.Blob
		if err == nil {
			client := services.GlobalBeaconService.GetIndexer().GetReadyClient(false, nil, nil)
			blobData, err = services.GlobalBeaconService.GetIndexer().BlobStore.LoadBlob(commitment, pageData.Block.BlockRoot, client)
		}
		if err == nil && blobData != nil {
			var blobModel *models.SlotPageBlob
			for _, blob := range pageData.Block.Blobs {
				if bytes.Equal(blob.KzgCommitment, commitment) {
					blobModel = blob
					break
				}
			}
			if blobModel != nil {
				blobModel.KzgProof = blobData.Proof
				if blobData.Blob != nil {
					blobModel.HaveData = true
					blobModel.Blob = *blobData.Blob
					if len(blobModel.Blob) > 512 {
						blobModel.BlobShort = blobModel.Blob[0:512]
						blobModel.IsShort = true
					} else {
						blobModel.BlobShort = blobModel.Blob
					}
				}
			}
		}
	}

	template := templates.GetTemplate(slotTemplateFiles...)
	data := InitPageData(w, r, "blockchain", "/slots", fmt.Sprintf("Slot %v", slotOrHash), slotTemplateFiles)
	data.Data = pageData
	w.Header().Set("Content-Type", "text/html")
	if handleTemplateError(w, r, "index.go", "Slot", "", template.ExecuteTemplate(w, "layout", data)) != nil {
		return // an error has occurred and was processed
	}
}

// SlotBlob handles responses for the block blobs tab
func SlotBlob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	commitment, err := hex.DecodeString(strings.Replace(vars["commitment"], "0x", "", -1))
	if err != nil || len(commitment) != 48 {
		http.Error(w, "Internal server error", http.StatusServiceUnavailable)
		return
	}

	blockRoot, err := hex.DecodeString(strings.Replace(vars["root"], "0x", "", -1))
	if err != nil || len(blockRoot) != 32 {
		http.Error(w, "Internal server error", http.StatusServiceUnavailable)
		return
	}

	client := services.GlobalBeaconService.GetIndexer().GetReadyClient(false, nil, nil)
	blobData, err := services.GlobalBeaconService.GetIndexer().BlobStore.LoadBlob(commitment, blockRoot, client)
	if err != nil {
		logrus.WithError(err).Error("error loading blob data")
		http.Error(w, "Internal server error", http.StatusServiceUnavailable)
		return
	}
	result := &models.SlotPageBlobDetails{
		KzgCommitment: fmt.Sprintf("%x", blobData.Commitment),
		KzgProof:      fmt.Sprintf("%x", blobData.Proof),
	}
	if blobData.Blob != nil {
		result.Blob = fmt.Sprintf("%x", *blobData.Blob)
	}
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		logrus.WithError(err).Error("error encoding blob sidecar")
		http.Error(w, "Internal server error", http.StatusServiceUnavailable)
	}
}

func getSlotPageData(blockSlot int64, blockRoot []byte, loadDuties bool) (*models.SlotPageData, error) {
	pageData := &models.SlotPageData{}
	pageCacheKey := fmt.Sprintf("slot:%v:%x:%v", blockSlot, blockRoot, loadDuties)
	pageRes, pageErr := services.GlobalFrontendCache.ProcessCachedPage(pageCacheKey, true, pageData, func(pageCall *services.FrontendCacheProcessingPage) interface{} {
		pageData, cacheTimeout := buildSlotPageData(blockSlot, blockRoot, loadDuties)
		pageCall.CacheTimeout = cacheTimeout
		return pageData
	})
	if pageErr == nil && pageRes != nil {
		resData, resOk := pageRes.(*models.SlotPageData)
		if !resOk {
			return nil, ErrInvalidPageModel
		}
		pageData = resData
	}
	return pageData, pageErr
}

func buildSlotPageData(blockSlot int64, blockRoot []byte, loadDuties bool) (*models.SlotPageData, time.Duration) {
	currentSlot := utils.TimeToSlot(uint64(time.Now().Unix()))
	finalizedEpoch, _ := services.GlobalBeaconService.GetFinalizedEpoch()
	var blockData *services.CombinedBlockResponse
	var err error
	if blockSlot > -1 {
		if uint64(blockSlot) <= currentSlot {
			blockData, err = services.GlobalBeaconService.GetSlotDetailsBySlot(uint64(blockSlot))
		}
	} else {
		blockData, err = services.GlobalBeaconService.GetSlotDetailsByBlockroot(blockRoot)
	}

	if err == nil {
		if blockData == nil {
			// check for orphaned block
			if blockSlot > -1 {
				dbBlocks := services.GlobalBeaconService.GetDbBlocksForSlots(uint64(blockSlot), 0, true)
				if len(dbBlocks) > 0 {
					blockRoot = dbBlocks[0].Root
				}
			}
			if blockRoot != nil {
				blockData = services.GlobalBeaconService.GetOrphanedBlock(blockRoot)
			}
		} else {
			// check orphaned status
			blockData.Orphaned = services.GlobalBeaconService.CheckBlockOrphanedStatus(blockData.Root)
		}
	}

	var slot uint64
	if blockData != nil {
		slot = uint64(blockData.Header.Message.Slot)
	} else if blockSlot > -1 {
		slot = uint64(blockSlot)
	} else {
		return nil, -1
	}
	logrus.Debugf("slot page called: %v", slot)

	pageData := &models.SlotPageData{
		Slot:           slot,
		Epoch:          utils.EpochOfSlot(slot),
		Ts:             utils.SlotToTime(slot),
		NextSlot:       slot + 1,
		PreviousSlot:   slot - 1,
		Future:         slot >= currentSlot,
		EpochFinalized: finalizedEpoch >= int64(utils.EpochOfSlot(slot)),
	}

	var assignments *rpc.EpochAssignments
	if loadDuties && !utils.Config.Frontend.AllowDutyLoading {
		loadDuties = false
	}
	if loadDuties {
		assignmentsRsp, err := services.GlobalBeaconService.GetEpochAssignments(utils.EpochOfSlot(slot))
		if err != nil {
			logrus.Printf("assignments error: %v", err)
			// we can safely continue here. the UI is prepared to work without epoch duties, but fields related to the duties are not shown
		} else {
			assignments = assignmentsRsp
		}
	}

	var cacheTimeout time.Duration
	if pageData.Future {
		timeDiff := time.Until(pageData.Ts)
		if timeDiff > 10*time.Minute {
			cacheTimeout = 10 * time.Minute
		} else {
			cacheTimeout = timeDiff
		}
	} else if pageData.EpochFinalized {
		cacheTimeout = 30 * time.Minute
	} else if blockData != nil {
		cacheTimeout = 5 * time.Minute
	} else {
		cacheTimeout = 10 * time.Second
	}

	if blockData == nil {
		pageData.Status = uint16(models.SlotStatusMissed)
		pageData.Proposer = math.MaxInt64
		if assignments != nil {
			pageData.Proposer = assignments.ProposerAssignments[slot]
		} else if epochStats := services.GlobalBeaconService.GetIndexer().GetCachedEpochStats(utils.EpochOfSlot(slot)); epochStats != nil {
			if proposers := epochStats.TryGetProposerAssignments(); proposers != nil {
				pageData.Proposer = proposers[slot]
			}
		}
		if pageData.Proposer == math.MaxInt64 {
			if assignment := db.GetSlotAssignment(slot); assignment != nil {
				pageData.Proposer = assignment.Proposer
			}
		}
		pageData.ProposerName = services.GlobalBeaconService.GetValidatorName(pageData.Proposer)
	} else {
		if blockData.Orphaned {
			pageData.Status = uint16(models.SlotStatusOrphaned)
		} else {
			pageData.Status = uint16(models.SlotStatusFound)
		}
		pageData.Proposer = uint64(blockData.Header.Message.ProposerIndex)
		pageData.ProposerName = services.GlobalBeaconService.GetValidatorName(pageData.Proposer)
		pageData.Block = getSlotPageBlockData(blockData, assignments, loadDuties)
	}

	return pageData, cacheTimeout
}

func getSlotPageBlockData(blockData *services.CombinedBlockResponse, assignments *rpc.EpochAssignments, loadDuties bool) *models.SlotPageBlockData {
	graffiti, _ := blockData.Block.Graffiti()
	randaoReveal, _ := blockData.Block.RandaoReveal()
	eth1Data, _ := blockData.Block.ETH1Data()
	attestations, _ := blockData.Block.Attestations()
	deposits, _ := blockData.Block.Deposits()
	voluntaryExits, _ := blockData.Block.VoluntaryExits()
	attesterSlashings, _ := blockData.Block.AttesterSlashings()
	proposerSlashings, _ := blockData.Block.ProposerSlashings()
	blsToExecChanges, _ := blockData.Block.BLSToExecutionChanges()
	syncAggregate, _ := blockData.Block.SyncAggregate()
	executionWithdrawals, _ := blockData.Block.Withdrawals()
	blobKzgCommitments, _ := blockData.Block.BlobKZGCommitments()

	pageData := &models.SlotPageBlockData{
		BlockRoot:              blockData.Root,
		ParentRoot:             blockData.Header.Message.ParentRoot[:],
		StateRoot:              blockData.Header.Message.StateRoot[:],
		Signature:              blockData.Header.Signature[:],
		RandaoReveal:           randaoReveal[:],
		Graffiti:               graffiti[:],
		Eth1dataDepositroot:    eth1Data.DepositRoot[:],
		Eth1dataDepositcount:   eth1Data.DepositCount,
		Eth1dataBlockhash:      eth1Data.BlockHash,
		ProposerSlashingsCount: uint64(len(proposerSlashings)),
		AttesterSlashingsCount: uint64(len(attesterSlashings)),
		AttestationsCount:      uint64(len(attestations)),
		DepositsCount:          uint64(len(deposits)),
		VoluntaryExitsCount:    uint64(len(voluntaryExits)),
		SlashingsCount:         uint64(len(proposerSlashings)) + uint64(len(attesterSlashings)),
		DutiesLoaded:           loadDuties,
		DenyDutyLoading:        !utils.Config.Frontend.AllowDutyLoading,
	}

	epoch := utils.EpochOfSlot(uint64(blockData.Header.Message.Slot))
	assignmentsMap := make(map[uint64]*rpc.EpochAssignments)
	assignmentsLoaded := make(map[uint64]bool)
	assignmentsMap[epoch] = assignments
	assignmentsLoaded[epoch] = true

	pageData.Attestations = make([]*models.SlotPageAttestation, pageData.AttestationsCount)
	for i, attestation := range attestations {
		var attAssignments []uint64
		attEpoch := utils.EpochOfSlot(uint64(attestation.Data.Slot))
		if !assignmentsLoaded[attEpoch] && loadDuties { // load epoch duties if needed
			attEpochAssignments, _ := services.GlobalBeaconService.GetEpochAssignments(attEpoch)
			assignmentsMap[attEpoch] = attEpochAssignments
			assignmentsLoaded[attEpoch] = true
		}

		if assignmentsMap[attEpoch] != nil {
			attAssignments = assignmentsMap[attEpoch].AttestorAssignments[fmt.Sprintf("%v-%v", uint64(attestation.Data.Slot), uint64(attestation.Data.Index))]
		} else {
			attAssignments = []uint64{}
		}
		attPageData := models.SlotPageAttestation{
			Slot:            uint64(attestation.Data.Slot),
			CommitteeIndex:  uint64(attestation.Data.Index),
			AggregationBits: attestation.AggregationBits,
			Validators:      make([]types.NamedValidator, len(attAssignments)),
			Signature:       attestation.Signature[:],
			BeaconBlockRoot: attestation.Data.BeaconBlockRoot[:],
			SourceEpoch:     uint64(attestation.Data.Source.Epoch),
			SourceRoot:      attestation.Data.Source.Root[:],
			TargetEpoch:     uint64(attestation.Data.Target.Epoch),
			TargetRoot:      attestation.Data.Target.Root[:],
		}
		for j := 0; j < len(attAssignments); j++ {
			attPageData.Validators[j] = types.NamedValidator{
				Index: attAssignments[j],
				Name:  services.GlobalBeaconService.GetValidatorName(attAssignments[j]),
			}
		}
		pageData.Attestations[i] = &attPageData
	}

	pageData.Deposits = make([]*models.SlotPageDeposit, pageData.DepositsCount)
	for i, deposit := range deposits {
		pageData.Deposits[i] = &models.SlotPageDeposit{
			PublicKey:             deposit.Data.PublicKey[:],
			Withdrawalcredentials: deposit.Data.WithdrawalCredentials,
			Amount:                uint64(deposit.Data.Amount),
			Signature:             deposit.Data.Signature[:],
		}
	}

	pageData.VoluntaryExits = make([]*models.SlotPageVoluntaryExit, pageData.VoluntaryExitsCount)
	for i, exit := range voluntaryExits {
		pageData.VoluntaryExits[i] = &models.SlotPageVoluntaryExit{
			ValidatorIndex: uint64(exit.Message.ValidatorIndex),
			ValidatorName:  services.GlobalBeaconService.GetValidatorName(uint64(exit.Message.ValidatorIndex)),
			Epoch:          uint64(exit.Message.Epoch),
			Signature:      exit.Signature[:],
		}
	}

	pageData.AttesterSlashings = make([]*models.SlotPageAttesterSlashing, pageData.AttesterSlashingsCount)
	for i, slashing := range attesterSlashings {
		slashingData := &models.SlotPageAttesterSlashing{
			Attestation1Indices:         make([]uint64, len(slashing.Attestation1.AttestingIndices)),
			Attestation1Signature:       slashing.Attestation1.Signature[:],
			Attestation1Slot:            uint64(slashing.Attestation1.Data.Slot),
			Attestation1Index:           uint64(slashing.Attestation1.Data.Index),
			Attestation1BeaconBlockRoot: slashing.Attestation1.Data.BeaconBlockRoot[:],
			Attestation1SourceEpoch:     uint64(slashing.Attestation1.Data.Source.Epoch),
			Attestation1SourceRoot:      slashing.Attestation1.Data.Source.Root[:],
			Attestation1TargetEpoch:     uint64(slashing.Attestation1.Data.Target.Epoch),
			Attestation1TargetRoot:      slashing.Attestation1.Data.Target.Root[:],
			Attestation2Indices:         make([]uint64, len(slashing.Attestation2.AttestingIndices)),
			Attestation2Signature:       slashing.Attestation2.Signature[:],
			Attestation2Slot:            uint64(slashing.Attestation2.Data.Slot),
			Attestation2Index:           uint64(slashing.Attestation2.Data.Index),
			Attestation2BeaconBlockRoot: slashing.Attestation2.Data.BeaconBlockRoot[:],
			Attestation2SourceEpoch:     uint64(slashing.Attestation2.Data.Source.Epoch),
			Attestation2SourceRoot:      slashing.Attestation2.Data.Source.Root[:],
			Attestation2TargetEpoch:     uint64(slashing.Attestation2.Data.Target.Epoch),
			Attestation2TargetRoot:      slashing.Attestation2.Data.Target.Root[:],
			SlashedValidators:           make([]types.NamedValidator, 0),
		}
		pageData.AttesterSlashings[i] = slashingData
		for j := range slashing.Attestation1.AttestingIndices {
			slashingData.Attestation1Indices[j] = uint64(slashing.Attestation1.AttestingIndices[j])
		}
		for j := range slashing.Attestation2.AttestingIndices {
			slashingData.Attestation2Indices[j] = uint64(slashing.Attestation2.AttestingIndices[j])
		}
		inter := intersect.Simple(slashing.Attestation1.AttestingIndices, slashing.Attestation2.AttestingIndices)
		for _, j := range inter {
			valIdx := j.(uint64)
			slashingData.SlashedValidators = append(slashingData.SlashedValidators, types.NamedValidator{
				Index: valIdx,
				Name:  services.GlobalBeaconService.GetValidatorName(valIdx),
			})
		}
	}

	pageData.ProposerSlashings = make([]*models.SlotPageProposerSlashing, pageData.ProposerSlashingsCount)
	for i, slashing := range proposerSlashings {
		pageData.ProposerSlashings[i] = &models.SlotPageProposerSlashing{
			ProposerIndex:     uint64(slashing.SignedHeader1.Message.ProposerIndex),
			ProposerName:      services.GlobalBeaconService.GetValidatorName(uint64(slashing.SignedHeader1.Message.ProposerIndex)),
			Header1Slot:       uint64(slashing.SignedHeader1.Message.Slot),
			Header1ParentRoot: slashing.SignedHeader1.Message.ParentRoot[:],
			Header1StateRoot:  slashing.SignedHeader1.Message.StateRoot[:],
			Header1BodyRoot:   slashing.SignedHeader1.Message.BodyRoot[:],
			Header1Signature:  slashing.SignedHeader1.Signature[:],
			Header2Slot:       uint64(slashing.SignedHeader2.Message.Slot),
			Header2ParentRoot: slashing.SignedHeader2.Message.ParentRoot[:],
			Header2StateRoot:  slashing.SignedHeader2.Message.StateRoot[:],
			Header2BodyRoot:   slashing.SignedHeader2.Message.BodyRoot[:],
			Header2Signature:  slashing.SignedHeader2.Signature[:],
		}
	}

	if epoch >= utils.Config.Chain.Config.AltairForkEpoch && syncAggregate != nil {
		pageData.SyncAggregateBits = syncAggregate.SyncCommitteeBits
		pageData.SyncAggregateSignature = syncAggregate.SyncCommitteeSignature[:]
		var syncAssignments []uint64
		if assignments != nil {
			syncAssignments = assignments.SyncAssignments
		} else if epochStats := services.GlobalBeaconService.GetIndexer().GetCachedEpochStats(epoch); epochStats != nil {
			syncAssignments = epochStats.TryGetSyncAssignments()
		}
		if syncAssignments == nil {
			syncPeriod := epoch / utils.Config.Chain.Config.EpochsPerSyncCommitteePeriod
			syncAssignments = db.GetSyncAssignmentsForPeriod(syncPeriod)
		}

		if len(syncAssignments) != 0 {
			pageData.SyncAggCommittee = make([]types.NamedValidator, len(syncAssignments))
			for idx, vidx := range syncAssignments {
				pageData.SyncAggCommittee[idx] = types.NamedValidator{
					Index: vidx,
					Name:  services.GlobalBeaconService.GetValidatorName(vidx),
				}
			}
		} else {
			pageData.SyncAggCommittee = []types.NamedValidator{}
		}
		pageData.SyncAggParticipation = utils.SyncCommitteeParticipation(pageData.SyncAggregateBits)
	}

	if epoch >= utils.Config.Chain.Config.BellatrixForkEpoch {
		switch blockData.Block.Version {
		case spec.DataVersionBellatrix:
			if blockData.Block.Bellatrix == nil {
				break
			}
			executionPayload := blockData.Block.Bellatrix.Message.Body.ExecutionPayload
			var baseFeePerGasBEBytes [32]byte
			for i := 0; i < 32; i++ {
				baseFeePerGasBEBytes[i] = executionPayload.BaseFeePerGas[32-1-i]
			}
			baseFeePerGas := new(big.Int).SetBytes(baseFeePerGasBEBytes[:])
			pageData.ExecutionData = &models.SlotPageExecutionData{
				ParentHash:    executionPayload.ParentHash[:],
				FeeRecipient:  executionPayload.FeeRecipient[:],
				StateRoot:     executionPayload.StateRoot[:],
				ReceiptsRoot:  executionPayload.ReceiptsRoot[:],
				LogsBloom:     executionPayload.LogsBloom[:],
				Random:        executionPayload.PrevRandao[:],
				GasLimit:      uint64(executionPayload.GasLimit),
				GasUsed:       uint64(executionPayload.GasUsed),
				Timestamp:     uint64(executionPayload.Timestamp),
				Time:          time.Unix(int64(executionPayload.Timestamp), 0),
				ExtraData:     executionPayload.ExtraData,
				BaseFeePerGas: baseFeePerGas.Uint64(),
				BlockHash:     executionPayload.BlockHash[:],
				BlockNumber:   uint64(executionPayload.BlockNumber),
			}
			getSlotPageTransactions(pageData, executionPayload.Transactions)
		case spec.DataVersionCapella:
			if blockData.Block.Capella == nil {
				break
			}
			executionPayload := blockData.Block.Capella.Message.Body.ExecutionPayload
			var baseFeePerGasBEBytes [32]byte
			for i := 0; i < 32; i++ {
				baseFeePerGasBEBytes[i] = executionPayload.BaseFeePerGas[32-1-i]
			}
			baseFeePerGas := new(big.Int).SetBytes(baseFeePerGasBEBytes[:])
			pageData.ExecutionData = &models.SlotPageExecutionData{
				ParentHash:    executionPayload.ParentHash[:],
				FeeRecipient:  executionPayload.FeeRecipient[:],
				StateRoot:     executionPayload.StateRoot[:],
				ReceiptsRoot:  executionPayload.ReceiptsRoot[:],
				LogsBloom:     executionPayload.LogsBloom[:],
				Random:        executionPayload.PrevRandao[:],
				GasLimit:      uint64(executionPayload.GasLimit),
				GasUsed:       uint64(executionPayload.GasUsed),
				Timestamp:     uint64(executionPayload.Timestamp),
				Time:          time.Unix(int64(executionPayload.Timestamp), 0),
				ExtraData:     executionPayload.ExtraData,
				BaseFeePerGas: baseFeePerGas.Uint64(),
				BlockHash:     executionPayload.BlockHash[:],
				BlockNumber:   uint64(executionPayload.BlockNumber),
			}
			getSlotPageTransactions(pageData, executionPayload.Transactions)
		case spec.DataVersionDeneb:
			if blockData.Block.Deneb == nil {
				break
			}
			executionPayload := blockData.Block.Deneb.Message.Body.ExecutionPayload
			pageData.ExecutionData = &models.SlotPageExecutionData{
				ParentHash:    executionPayload.ParentHash[:],
				FeeRecipient:  executionPayload.FeeRecipient[:],
				StateRoot:     executionPayload.StateRoot[:],
				ReceiptsRoot:  executionPayload.ReceiptsRoot[:],
				LogsBloom:     executionPayload.LogsBloom[:],
				Random:        executionPayload.PrevRandao[:],
				GasLimit:      uint64(executionPayload.GasLimit),
				GasUsed:       uint64(executionPayload.GasUsed),
				Timestamp:     uint64(executionPayload.Timestamp),
				Time:          time.Unix(int64(executionPayload.Timestamp), 0),
				ExtraData:     executionPayload.ExtraData,
				BaseFeePerGas: executionPayload.BaseFeePerGas.Uint64(),
				BlockHash:     executionPayload.BlockHash[:],
				BlockNumber:   uint64(executionPayload.BlockNumber),
			}
			getSlotPageTransactions(pageData, executionPayload.Transactions)
		case spec.DataVersionVerkle:
			if blockData.Block.Verkle == nil {
				break
			}
			executionPayload := blockData.Block.Verkle.Message.Body.ExecutionPayload
			var baseFeePerGasBEBytes [32]byte
			for i := 0; i < 32; i++ {
				baseFeePerGasBEBytes[i] = executionPayload.BaseFeePerGas[32-1-i]
			}
			baseFeePerGas := new(big.Int).SetBytes(baseFeePerGasBEBytes[:])
			pageData.ExecutionData = &models.SlotPageExecutionData{
				ParentHash:    executionPayload.ParentHash[:],
				FeeRecipient:  executionPayload.FeeRecipient[:],
				StateRoot:     executionPayload.StateRoot[:],
				ReceiptsRoot:  executionPayload.ReceiptsRoot[:],
				LogsBloom:     executionPayload.LogsBloom[:],
				Random:        executionPayload.PrevRandao[:],
				GasLimit:      uint64(executionPayload.GasLimit),
				GasUsed:       uint64(executionPayload.GasUsed),
				Timestamp:     uint64(executionPayload.Timestamp),
				Time:          time.Unix(int64(executionPayload.Timestamp), 0),
				ExtraData:     executionPayload.ExtraData,
				BaseFeePerGas: baseFeePerGas.Uint64(),
				BlockHash:     executionPayload.BlockHash[:],
				BlockNumber:   uint64(executionPayload.BlockNumber),
			}
			pageData.ExecutionWitness = &models.SlotPageExecutionWitness{Witness: executionPayload.ExecutionWitness}
			getSlotPageTransactions(pageData, executionPayload.Transactions)
		}
	}

	if epoch >= utils.Config.Chain.Config.CappellaForkEpoch {
		pageData.BLSChangesCount = uint64(len(blsToExecChanges))
		pageData.BLSChanges = make([]*models.SlotPageBLSChange, pageData.BLSChangesCount)
		for i, blschange := range blsToExecChanges {
			pageData.BLSChanges[i] = &models.SlotPageBLSChange{
				ValidatorIndex: uint64(blschange.Message.ValidatorIndex),
				ValidatorName:  services.GlobalBeaconService.GetValidatorName(uint64(blschange.Message.ValidatorIndex)),
				BlsPubkey:      []byte(blschange.Message.FromBLSPubkey[:]),
				Address:        []byte(blschange.Message.ToExecutionAddress[:]),
				Signature:      []byte(blschange.Signature[:]),
			}
		}

		pageData.WithdrawalsCount = uint64(len(executionWithdrawals))
		pageData.Withdrawals = make([]*models.SlotPageWithdrawal, pageData.WithdrawalsCount)
		for i, withdrawal := range executionWithdrawals {
			pageData.Withdrawals[i] = &models.SlotPageWithdrawal{
				Index:          uint64(withdrawal.Index),
				ValidatorIndex: uint64(withdrawal.ValidatorIndex),
				ValidatorName:  services.GlobalBeaconService.GetValidatorName(uint64(withdrawal.ValidatorIndex)),
				Address:        withdrawal.Address[:],
				Amount:         uint64(withdrawal.Amount),
			}
		}
	}

	if epoch >= utils.Config.Chain.Config.DenebForkEpoch {
		pageData.BlobsCount = uint64(len(blobKzgCommitments))
		pageData.Blobs = make([]*models.SlotPageBlob, pageData.BlobsCount)
		for i := range blobKzgCommitments {
			blobData := &models.SlotPageBlob{
				Index:         uint64(i),
				KzgCommitment: blobKzgCommitments[i][:],
			}
			pageData.Blobs[i] = blobData
		}
	}

	return pageData
}

func getSlotPageTransactions(pageData *models.SlotPageBlockData, tranactions []bellatrix.Transaction) {
	pageData.Transactions = make([]*models.SlotPageTransaction, 0)
	sigLookupBytes := []types.TxSignatureBytes{}
	sigLookupMap := map[types.TxSignatureBytes][]*models.SlotPageTransaction{}

	for idx, txBytes := range tranactions {
		var tx ethtypes.Transaction

		err := tx.UnmarshalBinary(txBytes)
		if err != nil {
			logrus.Warnf("error decoding transaction 0x%x.%v: %v\n", pageData.BlockRoot, idx, err)
			continue
		}

		txHash := tx.Hash()
		txValue, _ := tx.Value().Float64()
		ethFloat, _ := utils.ETH.Float64()
		txValue = txValue / ethFloat

		txData := &models.SlotPageTransaction{
			Index: uint64(idx),
			Hash:  txHash[:],
			Value: txValue,
			Data:  tx.Data(),
			Type:  uint64(tx.Type()),
		}
		txData.DataLen = uint64(len(txData.Data))
		txFrom, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(tx.ChainId()), &tx)
		if err != nil {
			txData.From = "unknown"
			logrus.Warnf("error decoding transaction sender 0x%x.%v: %v\n", pageData.BlockRoot, idx, err)
		} else {
			txData.From = txFrom.String()
		}
		txTo := tx.To()
		if txTo == nil {
			txData.To = "new contract"
		} else {
			txData.To = txTo.String()
		}

		pageData.Transactions = append(pageData.Transactions, txData)

		// check call fn signature
		if txData.DataLen >= 4 {
			sigBytes := types.TxSignatureBytes(txData.Data[0:4])
			if sigLookupMap[sigBytes] == nil {
				sigLookupMap[sigBytes] = []*models.SlotPageTransaction{
					txData,
				}
				sigLookupBytes = append(sigLookupBytes, sigBytes)
			} else {
				sigLookupMap[sigBytes] = append(sigLookupMap[sigBytes], txData)
			}
		} else {
			txData.FuncSigStatus = 10
			txData.FuncName = "transfer"
		}
	}
	pageData.TransactionsCount = uint64(len(tranactions))

	if len(sigLookupBytes) > 0 {
		sigLookups := services.GlobalTxSignaturesService.LookupSignatures(sigLookupBytes)
		for _, sigLookup := range sigLookups {
			for _, txData := range sigLookupMap[sigLookup.Bytes] {
				txData.FuncSigStatus = uint64(sigLookup.Status)
				txData.FuncBytes = fmt.Sprintf("0x%x", sigLookup.Bytes[:])
				if sigLookup.Status == types.TxSigStatusFound {
					txData.FuncSig = sigLookup.Signature
					txData.FuncName = sigLookup.Name
				} else {
					txData.FuncName = "call?"
				}
			}
		}
	}
}
