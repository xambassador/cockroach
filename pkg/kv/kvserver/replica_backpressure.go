// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package kvserver

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
)

var backpressureLogLimiter = log.Every(500 * time.Millisecond)

// backpressureRangeHardCap is a hard cap on the absolute size a range is
// allowed to grow to before backpressure will be applied to all writes. This
// absolute value takes precedence over any user-defined zone configuration
// value in conjunction with the backpressureRangeSizeMultiplier. It is intended
// to be the last defense against absurdly large ranges, in cases where
// backpressure is disabled by setting backpressureRangeSizeMultiplier to 0, or
// doesn't apply due to the backpressureByteTolerance[1], or a user has
// fat-fingered a zone configuration.
//
// [1] See comment on backpressureByteTolerance about the risk of disabling
// backpressure with that setting.
var backpressureRangeHardCap = settings.RegisterByteSizeSetting(
	settings.SystemOnly,
	"kv.range.range_size_hard_cap",
	"hard cap on the maximum size a range is allowed to grow to without"+
		"splitting before writes to the range are blocked. Takes precedence over all other configurations",
	8<<30, /* 8 GiB */
	settings.ByteSizeWithMinimum(64<<20 /* 64 MiB */),
	settings.WithPublic,
)

// backpressureRangeSizeMultiplier is the multiple of range_max_bytes that a
// range's size must grow to before backpressure will be applied on writes. Set
// to 0 to disable backpressure altogether.
var backpressureRangeSizeMultiplier = settings.RegisterFloatSetting(settings.SystemOnly,
	"kv.range.backpressure_range_size_multiplier",
	"multiple of range_max_bytes that a range is allowed to grow to without "+
		"splitting before writes to that range are blocked, or 0 to disable",
	2.0,
	settings.FloatWithMinimumOrZeroDisable(1),
)

// backpressureByteTolerance exists to deal with the fact that lowering the
// range size by anything larger than the backpressureRangeSizeMultiplier would
// immediately mean that all ranges require backpressure. To mitigate this
// unwanted backpressure we say that any range which is larger than the
// size where backpressure would kick in by more than this quantity will
// immediately avoid backpressure. This approach is a bit risky because a
// command larger than this value would effectively disable backpressure
// altogether. Another downside of this approach is that if the range size
// is reduced by roughly exactly the multiplier then we'd potentially have
// lots of ranges in this state.
//
// We additionally mitigate this situation further by doing the following:
//
//  1. We store in-memory on each replica the largest zone configuration range
//     size (largestPreviousMaxRangeBytes) we've seen and we do not backpressure
//     if the current range size is less than that. That value is cleared when
//     a range splits or runs GC such that the range size becomes smaller than
//     the current max range size. This mitigation alone is insufficient because
//     a node may restart before the splitting has concluded, leaving the
//     cluster in a state of backpressure.
//
//  2. We assign a higher priority in the snapshot queue to ranges which are
//     currently backpressuring than ranges which are larger but are not
//     applying backpressure.
var backpressureByteTolerance = settings.RegisterByteSizeSetting(
	settings.SystemOnly,
	"kv.range.backpressure_byte_tolerance",
	"defines the number of bytes above the product of "+
		"backpressure_range_size_multiplier and the range_max_size at which "+
		"backpressure will not apply",
	32<<20 /* 32 MiB */)

// backpressurableSpans contains spans of keys where write backpressuring
// is permitted. Writes to any keys within these spans may cause a batch
// to be backpressured.
var backpressurableSpans = []roachpb.Span{
	{Key: keys.TimeseriesPrefix, EndKey: keys.TimeseriesKeyMax},
	// Backpressure from the end of the system config forward instead of
	// over all table data to avoid backpressuring unsplittable ranges.
	{Key: keys.SystemConfigTableDataMax, EndKey: keys.TableDataMax},
}

// canBackpressureBatch returns whether the provided BatchRequest is eligible
// for backpressure.
func canBackpressureBatch(ba *kvpb.BatchRequest) bool {
	// Don't backpressure splits themselves.
	if ba.Txn != nil && ba.Txn.Name == splitTxnName {
		return false
	}

	// Only backpressure batches containing a "backpressurable"
	// method that is within a "backpressurable" key span.
	for _, ru := range ba.Requests {
		req := ru.GetInner()
		if !kvpb.CanBackpressure(req) {
			continue
		}

		for _, s := range backpressurableSpans {
			if s.Contains(req.Header().Span()) {
				return true
			}
		}
	}
	return false
}

// signallerForBatch returns the signaller to use for this batch in the
// following priorities:
// 1. If the batch contains a request uses poison.Policy_Wait, we will return
// neverTripSignaller.
// 2. If the replica is leaderless for a time longer than the threshold in
// `kv.replica_raft.leaderless_unavailable_threshold`, use the
// leaderlessWatcher signal.
// 3. Otherwise, use the replica's breaker's signaller
func (r *Replica) signallerForBatch(ba *kvpb.BatchRequest) signaller {
	for _, ru := range ba.Requests {
		req := ru.GetInner()
		if kvpb.BypassesReplicaCircuitBreaker(req) {
			return neverTripSignaller{}
		}
	}

	// If the leaderless watcher indicates that this replica is leaderless for a
	// long time, use it as the signal.
	if r.LeaderlessWatcher.IsUnavailable() {
		return r.LeaderlessWatcher
	}

	// Otherwise, use the replica's breaker.
	return r.breaker.Signal()
}

// shouldBackpressureWrites returns whether writes to the range should be
// subject to backpressure. This is based on the size of the range in
// relation to the split size. The method returns true if the range is more
// than backpressureRangeSizeMultiplier times larger than the split size but not
// larger than that by more than backpressureByteTolerance (see that comment for
// further explanation). It ensures that writes are always backpressured if the
// range's size is already larger than the absolute maximum we'll allow.
func (r *Replica) shouldBackpressureWrites() bool {
	r.mu.RLock()
	size := r.shMu.state.Stats.Total()
	rangeMaxBytes := r.mu.conf.RangeMaxBytes
	largestPreviousMaxRangeSize := r.mu.largestPreviousMaxRangeSizeBytes
	r.mu.RUnlock()

	// Check if the current range's size is already over the absolute maximum
	// we'll allow. Don't bother with any multipliers/byte tolerance calculations
	// if it is.
	rangeSizeHardCap := backpressureRangeHardCap.Get(&r.store.cfg.Settings.SV)

	if size >= rangeSizeHardCap {
		return true
	}

	mult := backpressureRangeSizeMultiplier.Get(&r.store.cfg.Settings.SV)
	if mult == 0 {
		// Disabled.
		return false
	}

	exceeded, bytesOver := exceedsMultipleOfSplitSize(mult, rangeMaxBytes,
		largestPreviousMaxRangeSize, size)
	if !exceeded {
		return false
	}
	if bytesOver > backpressureByteTolerance.Get(&r.store.cfg.Settings.SV) {
		return false
	}
	return true
}

// maybeBackpressureBatch blocks to apply backpressure if the replica deems
// that backpressure is necessary.
func (r *Replica) maybeBackpressureBatch(ctx context.Context, ba *kvpb.BatchRequest) error {
	if !canBackpressureBatch(ba) {
		return nil
	}

	// If we need to apply backpressure, wait for an ongoing split to finish
	// if one exists. This does not place a hard upper bound on the size of
	// a range because we don't track all in-flight requests (like we do for
	// the quota pool), but it does create an effective soft upper bound.
	for first := true; r.shouldBackpressureWrites(); first = false {
		if first {
			r.store.metrics.BackpressuredOnSplitRequests.Inc(1)
			defer r.store.metrics.BackpressuredOnSplitRequests.Dec(1) //nolint:deferloop

			if backpressureLogLimiter.ShouldLog() {
				log.Warningf(ctx, "applying backpressure to limit range growth on batch %s", ba)
			}
		}

		// Register a callback on an ongoing split for this range in the splitQueue.
		splitC := make(chan error, 1)
		if !r.store.splitQueue.MaybeAddCallback(r.RangeID, func(err error) {
			splitC <- err
		}) {
			// No split ongoing. We may have raced with its completion. There's
			// no good way to prevent this race, so we conservatively allow the
			// request to proceed instead of throwing an error that would surface
			// to the client.
			return nil
		}

		const errHint = `For help understanding this error and troubleshooting, visit:

    https://www.cockroachlabs.com/docs/stable/common-errors.html#split-failed-while-applying-backpressure-are-rows-updated-in-a-tight-loop`

		// Wait for the callback to be called.
		select {
		case <-ctx.Done():
			return errors.WithHint(errors.Wrapf(
				ctx.Err(), "aborted while applying backpressure to %s on range %s", ba, r.Desc(),
			), errHint)
		case err := <-splitC:
			if err != nil {
				return errors.WithHint(errors.Wrapf(
					err, "split failed while applying backpressure to %s on range %s", ba, r.Desc(),
				), errHint)
			}
		}
	}
	return nil
}
