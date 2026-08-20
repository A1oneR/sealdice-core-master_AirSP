package dice

import (
	"sort"
	"strconv"
	"strings"
)

var officialQQIdentityAliases SyncMap[string, string]

// RegisterOfficialQQIdentityAlias registers a compatibility-only identity
// mapping. Message routing continues to use OpenID; persistent user/group data
// may resolve through the legacy QQ identity.
func RegisterOfficialQQIdentityAlias(officialID, legacyID string) bool {
	officialID = strings.TrimSpace(officialID)
	legacyID = strings.TrimSpace(legacyID)
	if isOfficialQQUserAliasPair(officialID, legacyID) || isOfficialQQGroupAliasPair(officialID, legacyID) {
		officialQQIdentityAliases.Store(officialID, legacyID)
		return true
	}
	return false
}

func RemoveOfficialQQIdentityAlias(officialID string) {
	officialQQIdentityAliases.Delete(strings.TrimSpace(officialID))
}

func resolveOfficialQQIdentityAlias(identity string) string {
	if alias, ok := officialQQIdentityAliases.Load(identity); ok && alias != "" {
		return alias
	}
	return identity
}

// ResolveOfficialQQIdentityAlias exposes the storage identity associated with
// an Official QQ OpenID. Callers must keep using the original OpenID for live
// platform routing; the resolved value is only for persistent user/group data.
func ResolveOfficialQQIdentityAlias(identity string) string {
	return resolveOfficialQQIdentityAlias(strings.TrimSpace(identity))
}

// resolveLogStorageGroupID maps an Official QQ group to the legacy QQ group
// that owns its native SealDice logs. The live OpenID remains unchanged on
// GroupInfo and messages so platform routing keeps using the current endpoint.
func resolveLogStorageGroupID(groupID string) string {
	return resolveOfficialQQIdentityAlias(strings.TrimSpace(groupID))
}

// officialQQIdentityOwnerCandidates returns every storage owner that belongs
// to the same bound QQ identity. Older releases could create character cards
// before the OpenID-to-QQ alias existed, leaving those cards owned by an
// OpenID even though new cards are owned by QQ:<number>.
func officialQQIdentityOwnerCandidates(identity string) []string {
	identity = strings.TrimSpace(identity)
	canonical := resolveOfficialQQIdentityAlias(identity)
	if canonical == "" {
		return nil
	}

	aliases := make([]string, 0)
	officialQQIdentityAliases.Range(func(officialID, legacyID string) bool {
		if legacyID == canonical && officialID != canonical {
			aliases = append(aliases, officialID)
		}
		return true
	})
	sort.Strings(aliases)

	candidates := make([]string, 0, len(aliases)+2)
	seen := map[string]bool{}
	appendCandidate := func(candidate string) {
		if candidate == "" || seen[candidate] {
			return
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	appendCandidate(canonical)
	for _, alias := range aliases {
		appendCandidate(alias)
	}
	appendCandidate(identity)
	return candidates
}

// officialQQIdentityGroupCandidates returns every persisted group identity
// registered for the same legacy QQ group. This mirrors the character owner
// lookup and lets log recovery see records written before an Official QQ group
// alias was refreshed in the current process.
func officialQQIdentityGroupCandidates(identity string) []string {
	identity = strings.TrimSpace(identity)
	canonical := resolveOfficialQQIdentityAlias(identity)
	if canonical == "" {
		return nil
	}

	aliases := make([]string, 0)
	officialQQIdentityAliases.Range(func(officialID, legacyID string) bool {
		if legacyID == canonical && strings.HasPrefix(officialID, "OpenQQ-Group:") && officialID != canonical {
			aliases = append(aliases, officialID)
		}
		return true
	})
	sort.Strings(aliases)

	candidates := make([]string, 0, len(aliases)+2)
	seen := map[string]bool{}
	appendCandidate := func(candidate string) {
		if candidate == "" || seen[candidate] {
			return
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	appendCandidate(canonical)
	for _, alias := range aliases {
		appendCandidate(alias)
	}
	appendCandidate(identity)
	return candidates
}

func isOfficialQQUserAliasPair(officialID, legacyID string) bool {
	isOfficial := strings.HasPrefix(officialID, "OpenQQ:") ||
		strings.HasPrefix(officialID, "OpenQQCH:") ||
		strings.HasPrefix(officialID, "OpenQQ-Member-T:")
	return isOfficial && isLegacyQQUserIdentity(legacyID)
}

func isOfficialQQGroupAliasPair(officialID, legacyID string) bool {
	return strings.HasPrefix(officialID, "OpenQQ-Group:") && isLegacyQQGroupIdentity(legacyID)
}

func isLegacyQQUserIdentity(identity string) bool {
	return isNumericQQIdentity(identity, "QQ:")
}

func isLegacyQQGroupIdentity(identity string) bool {
	return isNumericQQIdentity(identity, "QQ-Group:")
}

func isNumericQQIdentity(identity, prefix string) bool {
	raw := strings.TrimPrefix(identity, prefix)
	if raw == identity || len(raw) < 5 || len(raw) > 12 {
		return false
	}
	_, err := strconv.ParseUint(raw, 10, 64)
	return err == nil
}
