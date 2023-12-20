package models

import (
	"time"

	"github.com/attestantio/go-eth2-client/spec/verkle"
	"github.com/pk910/dora/types"
)

// SlotPageData is a struct to hold info for the slot details page
type SlotPageData struct {
	Slot                   uint64             `json:"slot"`
	Epoch                  uint64             `json:"epoch"`
	EpochFinalized         bool               `json:"epoch_finalized"`
	EpochParticipationRate float64            `json:"epoch_participation_rate"`
	Ts                     time.Time          `json:"time"`
	NextSlot               uint64             `json:"next_slot"`
	PreviousSlot           uint64             `json:"prev_slot"`
	Status                 uint16             `json:"status"`
	Future                 bool               `json:"future"`
	Proposer               uint64             `json:"proposer"`
	ProposerName           string             `json:"proposer_name"`
	Block                  *SlotPageBlockData `json:"block"`
}

type SlotStatus uint16

const (
	SlotStatusMissed   SlotStatus = 0
	SlotStatusFound    SlotStatus = 1
	SlotStatusOrphaned SlotStatus = 2
)

type SlotPageBlockData struct {
	BlockRoot              []byte                 `json:"blockroot"`
	ParentRoot             []byte                 `json:"parentroot"`
	StateRoot              []byte                 `json:"stateroot"`
	Signature              []byte                 `json:"signature"`
	RandaoReveal           []byte                 `json:"randaoreveal"`
	Graffiti               []byte                 `json:"graffiti"`
	Eth1dataDepositroot    []byte                 `json:"eth1data_depositroot"`
	Eth1dataDepositcount   uint64                 `json:"eth1data_depositcount"`
	Eth1dataBlockhash      []byte                 `json:"eth1data_blockhash"`
	SyncAggregateBits      []byte                 `json:"syncaggregate_bits"`
	SyncAggregateSignature []byte                 `json:"syncaggregate_signature"`
	SyncAggParticipation   float64                `json:"syncaggregate_participation"`
	SyncAggCommittee       []types.NamedValidator `json:"syncaggregate_committee"`
	ProposerSlashingsCount uint64                 `json:"proposer_slashings_count"`
	AttesterSlashingsCount uint64                 `json:"attester_slashings_count"`
	AttestationsCount      uint64                 `json:"attestations_count"`
	DepositsCount          uint64                 `json:"deposits_count"`
	WithdrawalsCount       uint64                 `json:"withdrawals_count"`
	BLSChangesCount        uint64                 `json:"bls_changes_count"`
	VoluntaryExitsCount    uint64                 `json:"voluntaryexits_count"`
	SlashingsCount         uint64                 `json:"slashings_count"`
	BlobsCount             uint64                 `json:"blobs_count"`
	DenyDutyLoading        bool                   `json:"deny_duties"`
	DutiesLoaded           bool                   `json:"duties_loaded"`
	TransactionsCount      uint64                 `json:"transactions_count"`

	ExecutionData     *SlotPageExecutionData      `json:"execution_data"`
	Attestations      []*SlotPageAttestation      `json:"attestations"`       // Attestations included in this block
	Deposits          []*SlotPageDeposit          `json:"deposits"`           // Deposits included in this block
	VoluntaryExits    []*SlotPageVoluntaryExit    `json:"voluntary_exits"`    // Voluntary Exits included in this block
	AttesterSlashings []*SlotPageAttesterSlashing `json:"attester_slashings"` // Attester Slashings included in this block
	ProposerSlashings []*SlotPageProposerSlashing `json:"proposer_slashings"` // Proposer Slashings included in this block
	BLSChanges        []*SlotPageBLSChange        `json:"bls_changes"`        // BLSChanges included in this block
	Withdrawals       []*SlotPageWithdrawal       `json:"withdrawals"`        // Withdrawals included in this block
	Blobs             []*SlotPageBlob             `json:"blobs"`              // Blob sidecars included in this block
	Transactions      []*SlotPageTransaction      `json:"transactions"`       // Transactions included in this block
	ExecutionWitness  *SlotPageExecutionWitness   `json:"executionWitness"`
}

type SlotPageExecutionData struct {
	ParentHash    []byte    `json:"parent_hash"`
	FeeRecipient  []byte    `json:"fee_recipient"`
	StateRoot     []byte    `json:"state_root"`
	ReceiptsRoot  []byte    `json:"receipts_root"`
	LogsBloom     []byte    `json:"logs_bloom"`
	Random        []byte    `json:"random"`
	GasLimit      uint64    `json:"gas_limit"`
	GasUsed       uint64    `json:"gas_used"`
	Timestamp     uint64    `json:"timestamp"`
	Time          time.Time `json:"time"`
	ExtraData     []byte    `json:"extra_data"`
	BaseFeePerGas uint64    `json:"base_fee_per_gas"`
	BlockHash     []byte    `json:"block_hash"`
	BlockNumber   uint64    `json:"block_number"`
}

type SlotPageAttestation struct {
	Slot           uint64 `json:"slot"`
	CommitteeIndex uint64 `json:"committeeindex"`

	AggregationBits []byte                 `json:"aggregationbits"`
	Validators      []types.NamedValidator `json:"validators"`

	Signature []byte `json:"signature"`

	BeaconBlockRoot []byte `json:"beaconblockroot"`
	SourceEpoch     uint64 `json:"source_epoch"`
	SourceRoot      []byte `json:"source_root"`
	TargetEpoch     uint64 `json:"target_epoch"`
	TargetRoot      []byte `json:"target_root"`
}

type SlotPageDeposit struct {
	PublicKey             []byte `json:"publickey"`
	Withdrawalcredentials []byte `json:"withdrawalcredentials"`
	Amount                uint64 `json:"amount"`
	Signature             []byte `json:"signature"`
}

type SlotPageVoluntaryExit struct {
	ValidatorIndex uint64 `json:"validatorindex"`
	ValidatorName  string `json:"validatorname"`
	Epoch          uint64 `json:"epoch"`
	Signature      []byte `json:"signature"`
}

// BlockPageAttesterSlashing is a struct to hold data for attester slashings on the block page
type SlotPageAttesterSlashing struct {
	Attestation1Indices         []uint64               `json:"attestation1_indices"`
	Attestation1Signature       []byte                 `json:"attestation1_signature"`
	Attestation1Slot            uint64                 `json:"attestation1_slot"`
	Attestation1Index           uint64                 `json:"attestation1_index"`
	Attestation1BeaconBlockRoot []byte                 `json:"attestation1_beaconblockroot"`
	Attestation1SourceEpoch     uint64                 `json:"attestation1_source_epoch"`
	Attestation1SourceRoot      []byte                 `json:"attestation1_source_root"`
	Attestation1TargetEpoch     uint64                 `json:"attestation1_target_epoch"`
	Attestation1TargetRoot      []byte                 `json:"attestation1_target_root"`
	Attestation2Indices         []uint64               `json:"attestation2_indices"`
	Attestation2Signature       []byte                 `json:"attestation2_signature"`
	Attestation2Slot            uint64                 `json:"attestation2_slot"`
	Attestation2Index           uint64                 `json:"attestation2_index"`
	Attestation2BeaconBlockRoot []byte                 `json:"attestation2_beaconblockroot"`
	Attestation2SourceEpoch     uint64                 `json:"attestation2_source_epoch"`
	Attestation2SourceRoot      []byte                 `json:"attestation2_source_root"`
	Attestation2TargetEpoch     uint64                 `json:"attestation2_target_epoch"`
	Attestation2TargetRoot      []byte                 `json:"attestation2_target_root"`
	SlashedValidators           []types.NamedValidator `json:"validators"`
}

// BlockPageProposerSlashing is a struct to hold data for proposer slashings on the block page
type SlotPageProposerSlashing struct {
	ProposerIndex     uint64 `json:"proposerindex"`
	ProposerName      string `json:"proposername"`
	Header1Slot       uint64 `json:"header1_slot"`
	Header1ParentRoot []byte `json:"header1_parentroot"`
	Header1StateRoot  []byte `json:"header1_stateroot"`
	Header1BodyRoot   []byte `json:"header1_bodyroot"`
	Header1Signature  []byte `json:"header1_signature"`
	Header2Slot       uint64 `json:"header2_slot"`
	Header2ParentRoot []byte `json:"header2_parentroot"`
	Header2StateRoot  []byte `json:"header2_stateroot"`
	Header2BodyRoot   []byte `json:"header2_bodyroot"`
	Header2Signature  []byte `json:"header2_signature"`
}

type SlotPageBLSChange struct {
	ValidatorIndex uint64 `json:"validatorindex"`
	ValidatorName  string `json:"validatorname"`
	BlsPubkey      []byte `json:"pubkey"`
	Address        []byte `json:"address"`
	Signature      []byte `json:"signature"`
}

type SlotPageWithdrawal struct {
	Index          uint64 `json:"index"`
	ValidatorIndex uint64 `json:"validatorindex"`
	ValidatorName  string `json:"validatorname"`
	Address        []byte `json:"address"`
	Amount         uint64 `json:"amount"`
}

type SlotPageBlob struct {
	Index         uint64 `json:"index"`
	KzgCommitment []byte `json:"kzg_commitment"`
	HaveData      bool   `json:"have_data"`
	IsShort       bool   `json:"is_short"`
	BlobShort     []byte `json:"blob_short"`
	Blob          []byte `json:"blob"`
	KzgProof      []byte `json:"kzg_proof"`
}

type SlotPageBlobDetails struct {
	Index         uint64 `json:"index"`
	Blob          string `json:"blob"`
	KzgCommitment string `json:"kzg_commitment"`
	KzgProof      string `json:"kzg_proof"`
}

type SlotPageTransaction struct {
	Index         uint64  `json:"index"`
	Hash          []byte  `json:"hash"`
	From          string  `json:"from"`
	To            string  `json:"to"`
	Value         float64 `json:"value"`
	Data          []byte  `json:"data"`
	DataLen       uint64  `json:"datalen"`
	FuncSigStatus uint64  `json:"func_sig_status"`
	FuncBytes     string  `json:"func_bytes"`
	FuncName      string  `json:"func_name"`
	FuncSig       string  `json:"func_sig"`
	Type          uint64  `json:"type"`
}

type SlotPageExecutionWitness struct {
	Witness *verkle.ExecutionWitness `json:"execution_witness"`
}
