package jiffy

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v9/market"
	"github.com/filecoin-shipyard/boostly"
	"github.com/filecoin-shipyard/telefil"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
)

const (

	// Unknown signals that the replica status is not known due to error during verification. See Replica.LastError.
	Unknown ReplicaStatus = iota
	// Accepted signals that the deal proposal has been accepted by the provider and is in process of being executed.
	Accepted
	// Slashed signals that the deal corresponding to the replica has been slashed.
	Slashed
	// Expired signals that the deal corresponding to the replica has expired.
	Expired
	// Published signals that the deal corresponding to the replica has been published on chain.
	Published
	// Active signals that the deal corresponding to the replica is active.
	Active
)

var (
	_ Replicator = (*simpleReplicator)(nil)

	replicaStatusNames = map[ReplicaStatus]string{
		Unknown:   "unknown",
		Accepted:  "accepted",
		Slashed:   "slashed",
		Expired:   "expired",
		Published: "published",
		Active:    "active",
	}
)

type (
	Replicator interface {
		GetReplicas(context.Context, abi.PieceInfo) ([]Replica, error)
	}
	Replica struct {
		DealProposal       boostly.DealProposal
		LastProviderStatus *boostly.DealStatusResponse
		LastChainStatus    *telefil.StateMarketStorageDeal
		LastChecked        time.Time
		LastError          error
	}
	ReplicaStatus int

	simpleReplicator struct {
		j *Jiffy

		ctx    context.Context
		cancel context.CancelFunc

		segmentReplicasMutex sync.RWMutex
		segmentReplicas      map[cid.Cid]map[uuid.UUID]*Replica // TODO persist to disk
	}
)

func (rs ReplicaStatus) String() string {
	if name, named := replicaStatusNames[rs]; named {
		return name
	}
	return fmt.Sprintf("unnamed(%d)", rs)
}

func (r *Replica) Provider() address.Address {
	return r.DealProposal.ClientDealProposal.Proposal.Provider
}
func (r *Replica) PieceCID() cid.Cid {
	return r.DealProposal.ClientDealProposal.Proposal.PieceCID
}
func (r *Replica) MatchesProposalOnChain() bool {
	proposalOnChain := r.LastChainStatus.Proposal
	proposal := r.DealProposal.ClientDealProposal.Proposal
	return proposal.Client == proposalOnChain.Client &&
		proposal.PieceCID.Equals(proposalOnChain.PieceCID) &&
		proposal.Provider == proposalOnChain.Provider

}

func (r *Replica) Status(head abi.ChainEpoch) ReplicaStatus {
	switch {
	case r.LastProviderStatus == nil:
		return Accepted
	case r.LastChainStatus == nil:
		// TODO: We could return more detailed status from r.LastChainStatus
		//       Investigate what would be useful to return.
		return Accepted
	case !r.MatchesProposalOnChain():
		// TODO: Should we do something more here? if client deal proposal doesn't match the on-chain proposal it means
		//       the provider must have returned an incorrect deal ID in response to deal status request.
		//       In which chase, there should be some penalization?
		return Accepted
	case r.LastChainStatus.State.SlashEpoch > 0:
		return Slashed
	case r.LastChainStatus.Proposal.EndEpoch > head:
		return Expired
	case r.LastChainStatus.State.SectorStartEpoch <= 0:
		return Published
	case r.LastChainStatus.State.SectorStartEpoch > 0:
		return Active
	default:
		return Accepted
	}
}

func (r *Replica) Expiration(genesis time.Time) time.Time {
	return genesis.Add(builtin.EpochDurationSeconds * time.Second * time.Duration(r.DealProposal.ClientDealProposal.Proposal.EndEpoch))
}

func newSimpleReplicator(j *Jiffy) (*simpleReplicator, error) {
	r := &simpleReplicator{j: j}
	r.ctx, r.cancel = context.WithCancel(context.Background())
	return r, nil
}

func (r *simpleReplicator) Start(ctx context.Context) error {
	go r.replicate(r.ctx)
	go r.verify(r.ctx)
	select {
	case <-ctx.Done():
		r.cancel()
		return ctx.Err()
	default:
		return nil
	}
}
func (r *simpleReplicator) replicate(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.j.replicatorInterval.C:
		}
		segments, err := r.j.ListSegments(ctx)
		if err != nil {
			continue
		}

		head, err := r.j.fil.ChainHead(ctx)
		if err != nil {
			logger.Errorw("failed to execute replication cycle: failed to get chain head", "err", err)
			continue
		}

		var underReplicated []*Segment
		r.segmentReplicasMutex.RLock()
	NextSegment:
		for _, segment := range segments {
			replicas, ok := r.segmentReplicas[segment.Info.PieceCID]
			switch {
			case !ok, replicas == nil, len(replicas) == 0:
				underReplicated = append(underReplicated, segment)
			default:
				for _, replica := range replicas {
					switch replica.Status(head.Height) {
					case Slashed, Expired:
						// TODO check that the new replica does not end up on SPs that already have a replica of data.
						//      We need replication "affinity" and "anti-affinity" as a general concept.
						underReplicated = append(underReplicated, segment)
						continue NextSegment
					case Unknown:
						// TODO we want some grace period before we start a new replica.
					}
				}
			}
		}
		r.segmentReplicasMutex.RUnlock()
		pieces, _, err := packBestFit(underReplicated, 32*GiB, 1)
		if err != nil {
			continue
		}
		if len(pieces) <= 0 {
			continue
		}
		for _, piece := range pieces {
			sps, err := r.j.replicatorSpPicker(ctx, piece)
			if err != nil {
				continue
			}
			if len(sps) == 0 {
				continue
			}
			for _, sp := range sps {
				deal, err := r.j.dealer.Deal(ctx, piece, sp)
				if err != nil {
					continue
				}
				r.segmentReplicasMutex.Lock()
				// Add an entry to piece replicas map for each segment of the piece
				for _, segment := range piece.Segments {
					replicas := r.segmentReplicas[segment.Info.PieceCID]
					if replicas == nil {
						replicas = make(map[uuid.UUID]*Replica)
					}
					replicas[deal.DealUUID] = &Replica{
						DealProposal: *deal,
					}
					r.segmentReplicas[segment.Info.PieceCID] = replicas
				}
				r.segmentReplicasMutex.Unlock()
				// TODO we might need to store piece CID -> replica deal UUID for retrieval.
			}
		}
	}
}

func (r *simpleReplicator) verify(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.j.replicatorVerificationInterval.C:
		}

		// Deduplicate all deals by deal UUID, since a segment may be present across multiple deals.
		toCheck := make(map[uuid.UUID]Replica)
		r.segmentReplicasMutex.RLock()
		for _, replicas := range r.segmentReplicas {
			for id, replica := range replicas {
				if _, ok := toCheck[id]; ok {
					continue
				}
				toCheck[id] = *replica
			}
		}
		r.segmentReplicasMutex.RUnlock()

		// TODO check in parallel with some configurable degree of concurrency.
		for _, replica := range toCheck {

			replica.LastChecked = time.Now()
			replica.LastChainStatus = nil
			replica.LastProviderStatus = nil
			replica.LastError = nil

			// TODO clean up expired deals
			// TODO handle slashed deals

			info, err := r.j.fil.StateMinerInfo(ctx, replica.Provider())
			if err != nil {
				replica.LastError = fmt.Errorf("failed to get state miner info: %w", err)
				continue
			}
			if err := r.j.h.Connect(ctx, *info); err != nil {
				replica.LastError = fmt.Errorf("failed to connect to provider: %w", err)
				continue
			}
			replica.LastProviderStatus, err = boostly.GetDealStatus(ctx, r.j.h, info.ID, replica.DealProposal.DealUUID, r.j.wallet.Sign)
			if err != nil {
				replica.LastError = fmt.Errorf("failed to get deal status from provider: %w", err)
				continue
			}
			if replica.LastProviderStatus.DealStatus.PublishCid == nil {
				// Not published yet; nothing further to do.
				continue
			}

			replica.LastChainStatus, err = r.j.fil.StateMarketStorageDeal(ctx, replica.LastProviderStatus.DealStatus.ChainDealID)
			if err != nil {
				replica.LastError = fmt.Errorf("failed to get storage deal status from chain: %w", err)
				continue
			}

			onChainProposal := replica.LastChainStatus.Proposal
			originalProposal := replica.DealProposal.ClientDealProposal.Proposal
			if err := verifyProposalsMatch(originalProposal, onChainProposal); err != nil {
				replica.LastError = fmt.Errorf("on chain proposal for deal ID does not match the original proposal: %w", err)
				continue
			}
			// TODO check retrieval?
			// TODO move over to sector checks once FIP#730 has landed:
			//  - https://github.com/filecoin-project/FIPs/discussions/730
			//  - https://github.com/filecoin-project/builtin-actors/compare/master...anorth/prove-commit2
		}

		r.segmentReplicasMutex.Lock()
		for _, replicas := range r.segmentReplicas {
			for id, replica := range replicas {
				if checked, ok := toCheck[id]; ok {
					replica.LastChecked = checked.LastChecked
					replica.LastChainStatus = checked.LastChainStatus
					replica.LastProviderStatus = checked.LastProviderStatus
					replica.LastError = checked.LastError
				}
			}
		}
		r.segmentReplicasMutex.Unlock()
	}
}

func verifyProposalsMatch(original, onChain market.DealProposal) error {
	switch {
	case original.Provider != onChain.Provider:
		return fmt.Errorf("provider mismatch; expected '%s' but got on chain value: '%s'", original.Provider, onChain.Provider)
	case original.PieceCID != onChain.PieceCID:
		return fmt.Errorf("piece CID mismatch; expected '%s' but got on chain value: '%s'", original.PieceCID, onChain.PieceCID)
	case original.Client != onChain.Client:
		return fmt.Errorf("client mismatch; expected '%s' but got on chain value: '%s'", original.Client, onChain.Client)
	case original.StartEpoch != onChain.StartEpoch:
		return fmt.Errorf("start epoch mismatch; expected '%s' but got on chain value: '%s'", original.StartEpoch, onChain.StartEpoch)
	case original.EndEpoch != onChain.EndEpoch:
		return fmt.Errorf("end epoch mismatch; expected '%s' but got on chain value: '%s'", original.EndEpoch, onChain.EndEpoch)
	default:
		// TODO what else is worth checking?
		return nil
	}
}

func (r *simpleReplicator) GetReplicas(ctx context.Context, pi abi.PieceInfo) ([]Replica, error) {
	m, ok := r.segmentReplicas[pi.PieceCID]
	if !ok || m == nil {
		return nil, nil
	}
	deals := make([]Replica, 0, len(m))
	for _, deal := range m {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			deals = append(deals, *deal)
		}
	}
	return deals, nil
}

func (r *simpleReplicator) Shutdown(_ context.Context) error {
	r.cancel()
	r.j.replicatorInterval.Stop()
	r.j.replicatorVerificationInterval.Stop()
	return nil
}
