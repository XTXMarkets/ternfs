// Copyright 2025 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: GPL-2.0-or-later

package cleanup

import (
	"fmt"
	"time"
	"xtx/ternfs/msgs"
)

// Returns how many edges to remove according to the policy (as a prefix of the input).
//
// `edges` should be all the snapshot edges for a certain name, oldest edge first. This
// must include the current edge.
//
// If it returns N, edges[N:] will be well formed too in the sense above.
func edgesToRemove(dir msgs.InodeId, policy *msgs.SnapshotPolicy, now msgs.TernTime, edges []msgs.Edge, minEdgeAge time.Duration) int {
	if len(edges) == 0 {
		return 0
	}
	// check that there is at most one current edge
	for i := len(edges) - 1; i >= 0; i-- {
		if edges[i].Current && i < len(edges)-1 {
			panic(fmt.Errorf("found non-trailing current edge for dir %v", dir))
		}
	}
	// check that it's all the same name
	name := edges[0].Name
	for i := 0; i < len(edges); i++ {
		if edges[i].Name != name {
			panic(fmt.Errorf("got multiple names in edges (%v and %v)", name, edges[i].Name))
		}
	}
	// check that times are ascending, apart from shard 0 for which this is not
	// true after the merge
	for i := 1; i < len(edges); i++ {
		if dir.Shard() == 0 && edges[i].Current {
			continue
		}
		if edges[i].CreationTime < edges[i-1].CreationTime {
			panic(fmt.Errorf("non-monotonic creation times"))
		}
	}
	// Do not consider the trailing current edge, if present.
	lastNonCurrentEdge := len(edges) - 1
	if edges[lastNonCurrentEdge].Current {
		lastNonCurrentEdge--
	}
	// Index dividing edges, so that all all edges[i] i < firstGoodEdgeVersions should be
	// removed, while all edges[i] i >= firstGoodEdgeVersions should be kept.
	var firstGoodEdgeVersions int
	// Note that DeleteAfterVersions only affects the snapshot edges: we don't look at the
	// current edge at all.
	//
	// This means, for example, that if we have DeleteAfterVersions=5, after applying the policy
	// there might be 5 or 6 edges with a certain name, depending on whether the a current
	// edge for it exists.
	if policy.DeleteAfterVersions.Active() {
		versionNumber := 0
		for firstGoodEdgeVersions = lastNonCurrentEdge; firstGoodEdgeVersions >= 0; firstGoodEdgeVersions-- {
			edge := &edges[firstGoodEdgeVersions]
			if edge.Current {
				panic(fmt.Errorf("unexpected current edge: %v", edge))
			}
			// ignore deletes, we just want to keep the last N versions.
			if edge.TargetId.Id() == msgs.NULL_INODE_ID {
				continue
			}
			creationTime := edge.CreationTime.Time()
			if versionNumber >= int(policy.DeleteAfterVersions.Versions()) && now.Time().Sub(creationTime) > minEdgeAge {
				firstGoodEdgeVersions++
				break
			}
			versionNumber++
		}
	}
	var firstGoodEdgeTime int
	if policy.DeleteAfterTime.Active() {
		for firstGoodEdgeTime = lastNonCurrentEdge; firstGoodEdgeTime >= 0; firstGoodEdgeTime-- {
			edge := &edges[firstGoodEdgeTime]
			if edge.Current {
				panic(fmt.Errorf("unexpected current edge: %v", edge))
			}
			creationTime := edge.CreationTime.Time()
			if now.Time().Sub(creationTime) > max(policy.DeleteAfterTime.Time(), minEdgeAge) {
				// We found an edge that was created before the time we want.
				// However, this is not enough: consider the case where we
				// create a file, then delete it after 3 months. The owning
				// snapshot edge will be older than `DeleteAfterTime`, but
				// we still want to preserve it. What we really care is when
				// the edge was overridden (by a delete or something else).
				// We know that the overriding edge is not past deadline.
				// So we just set the first good edge to the current one.
				//
				// Note that this problem is also present for non-owned edges,
				// but in that case we can't rely on having a followup edge. But
				// we try regardless. See <https://eulergamma.slack.com/archives/C03PCJMGAAC/p1712144860299789>.
				if edge.TargetId.Extra() || (edge.TargetId.Id() != msgs.NULL_INODE_ID && firstGoodEdgeTime < len(edges)-1) {
					// note that if the edge is owned, we are asserting that a next edge exists, which
					// is exactly what we want.
					nextEdge := &edges[firstGoodEdgeTime+1]
					// This might be a current edge, i.e. not caught by this loop,
					// so we need another check here.
					if now.Time().Sub(nextEdge.CreationTime.Time()) > policy.DeleteAfterTime.Time() {
						// The next edge is expired, mark current edge as bad.
						firstGoodEdgeTime++
					}
				} else {
					// Mark the current edge as bad.
					firstGoodEdgeTime++
				}
				break
			}
		}
	}
	// Remove as much as possible (i.e. the rules are a disjuntion)
	firstGoodEdge := firstGoodEdgeVersions
	if firstGoodEdgeTime > firstGoodEdge {
		firstGoodEdge = firstGoodEdgeTime
	}
	if firstGoodEdge < 0 {
		firstGoodEdge = 0
	}
	// if the last edge is a delete, remove that too (we don't want to keep a delete hanging)
	if firstGoodEdge < len(edges) && edges[firstGoodEdge].TargetId.Id() == msgs.NULL_INODE_ID {
		firstGoodEdge++
	}
	return firstGoodEdge
}
