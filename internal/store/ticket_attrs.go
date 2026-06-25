package store

import "strings"

// ticket_attrs.go is the declare-once framework for Tier-2 (attrs-bag) ticket
// fields (TK-173, epic TK-171). See docs/design/extensible-schema.md §14.
//
// Before this, an attrs-backed scalar field was hand-wired in three parallel
// lists that had to stay in lockstep: a key list (ticketAttrStringKeys), the
// read side (hydrateTicketAttrs) and the write side (ticketAttrsForWrite). Miss
// one and the field silently fails to persist or load. Now each scalar field is
// declared ONCE in ticketAttrScalarFields below; hydrate and write are derived
// from that single declaration.
//
// Design: a reflection-free registry. Each field carries its attrs key plus
// hydrate/write closures built over a pointer-accessor to the typed Ticket
// field, so the same declaration drives both directions with no reflection and
// no per-call allocation (the closures are created once at package init).

// ticketAttrField is one declared attrs-backed scalar field on Ticket.
type ticketAttrField struct {
	key     string                   // the JSON key inside tickets.attrs
	hydrate func(t *Ticket, a Attrs) // bag -> typed field (read side)
	write   func(a Attrs, t *Ticket) // typed field -> bag, sparse (write side)
}

// strAttrField declares a string-valued attrs field. ref returns a pointer to
// the backing Ticket field so the same accessor serves both read and write.
// Empty values are not stored (SetString deletes), keeping the bag sparse.
func strAttrField(key string, ref func(*Ticket) *string) ticketAttrField {
	return ticketAttrField{
		key:     key,
		hydrate: func(t *Ticket, a Attrs) { *ref(t) = a.GetString(key) },
		write:   func(a Attrs, t *Ticket) { a.SetString(key, strings.TrimSpace(*ref(t))) },
	}
}

// intAttrField declares an int-valued attrs field. Zero is not stored (SetInt
// deletes), keeping the bag sparse.
func intAttrField(key string, ref func(*Ticket) *int) ticketAttrField {
	return ticketAttrField{
		key:     key,
		hydrate: func(t *Ticket, a Attrs) { *ref(t) = a.GetInt(key) },
		write:   func(a Attrs, t *Ticket) { a.SetInt(key, *ref(t)) },
	}
}

// ticketAttrScalarFields is the single source of truth for attrs-backed scalar
// ticket fields. To add a new Tier-2 scalar field: add the struct field + json
// tag to Ticket, then add ONE line here. No edits to hydrate/write/key lists.
// (Nested guidance maps dor_map/dod_map/ac_map have bespoke helpers and are
// handled alongside this registry in hydrateTicketAttrs/ticketAttrsForWrite.)
var ticketAttrScalarFields = []ticketAttrField{
	strAttrField("git_repository", func(t *Ticket) *string { return &t.GitRepository }),
	strAttrField("git_branch", func(t *Ticket) *string { return &t.GitBranch }),
	strAttrField("estimate_complete", func(t *Ticket) *string { return &t.EstimateComplete }),
	strAttrField("author", func(t *Ticket) *string { return &t.Author }),
	strAttrField("pr_url", func(t *Ticket) *string { return &t.PrURL }),
	intAttrField("health_score", func(t *Ticket) *int { return &t.HealthScore }),
}

// ticketAttrScalarKeys returns the declared scalar attrs keys (used by callers
// that need the key set, e.g. tests asserting EnsureAttrIndex coverage).
func ticketAttrScalarKeys() []string {
	keys := make([]string, len(ticketAttrScalarFields))
	for i, f := range ticketAttrScalarFields {
		keys[i] = f.key
	}
	return keys
}
