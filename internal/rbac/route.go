package rbac

import "strings"

// MatchRoute scans rules in order and returns the permission required for the
// first matching route. Returns "" when no rule matches (route is unrestricted).
func MatchRoute(rules []RouteRule, method, path string) string {
	for _, rule := range rules {
		if rule.Method != "*" && rule.Method != method {
			continue
		}
		if _, ok := matchAndCapture(rule.Path, path); ok {
			return rule.Permission
		}
	}
	return ""
}

// matchAndCapture matches pattern against path using the same glob semantics as
// fusion-bff: literal segments match exactly, * in non-last position matches one
// segment, trailing * matches one or more segments, <prefix>* matches segments
// starting with prefix.
func matchAndCapture(pattern, path string) (firstCapture string, matched bool) {
	pp := strings.Split(strings.Trim(pattern, "/"), "/")
	ap := strings.Split(strings.Trim(path, "/"), "/")

	captured := false
	for i, seg := range pp {
		last := i == len(pp)-1

		if seg == "*" {
			if last {
				if !captured && i < len(ap) {
					firstCapture = ap[i]
				}
				return firstCapture, i < len(ap)
			}
			if i >= len(ap) {
				return "", false
			}
			if !captured {
				firstCapture = ap[i]
				captured = true
			}
			continue
		}

		if strings.HasSuffix(seg, "*") {
			prefix := seg[:len(seg)-1]
			if i >= len(ap) || !strings.HasPrefix(ap[i], prefix) {
				return "", false
			}
			return firstCapture, true
		}

		if i >= len(ap) || ap[i] != seg {
			return "", false
		}
	}
	return firstCapture, len(ap) == len(pp)
}
