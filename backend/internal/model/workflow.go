package model

// StatusTransitions is the issue status workflow: each status maps to the
// statuses it may legally move to directly.
//
// It lives in model (rather than service) because both sides of the wire need
// it: the API service enforces it on PATCH /issues/{id}/status, and the mf CLI
// consults it to walk multi-hop transitions (see [TransitionPath]) instead of
// failing on an illegal single hop.
var StatusTransitions = map[string][]string{
	"todo":        {"in_progress", "suspended", "rejected"},
	"in_progress": {"review", "done", "suspended", "todo"},
	"review":      {"testing", "in_progress"},
	"testing":     {"done", "in_progress"},
	"done":        {"closed", "in_progress"},
	"suspended":   {"todo"},
	"rejected":    {"todo"},
}

// AllowedTransitions returns the statuses reachable in one step from status.
// The second result is false when status is not a known workflow state.
func AllowedTransitions(status string) ([]string, bool) {
	next, ok := StatusTransitions[status]
	return next, ok
}

// CanTransition reports whether from -> to is a single legal step.
func CanTransition(from, to string) bool {
	for _, s := range StatusTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// TransitionPath returns the shortest sequence of statuses to walk to get from
// `from` to `to`, excluding `from` and including `to` as the final element. A
// no-op (from == to) yields an empty path. ok is false when `to` is
// unreachable from `from`.
//
// This is what lets `mf issue done MF-1` succeed on a todo issue: the direct
// hop todo -> done is illegal, but todo -> in_progress -> done is not.
func TransitionPath(from, to string) (path []string, ok bool) {
	if from == to {
		return nil, true
	}
	if _, known := StatusTransitions[from]; !known {
		return nil, false
	}

	// Breadth-first search over the transition graph. Successors are visited in
	// their declared order so the chosen path is deterministic.
	prev := map[string]string{from: ""}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, next := range StatusTransitions[cur] {
			if _, seen := prev[next]; seen {
				continue
			}
			prev[next] = cur
			if next == to {
				// Walk the predecessor chain back to `from`, then reverse it.
				for at := to; at != from; at = prev[at] {
					path = append(path, at)
				}
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path, true
			}
			queue = append(queue, next)
		}
	}
	return nil, false
}
