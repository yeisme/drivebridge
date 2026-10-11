package service

// CreateAgentOperator issues an agent token with operator authority over
// the private state directory: the owner is derived from the target space
// row so the operator never guesses user IDs. Validation and token minting
// are identical to the HTTP surface.
func (s *Service) CreateAgentOperator(space, name, prefix, scopes, connection string, days int) (Agent, string, error) {
	var sp Space
	if s.DB.First(&sp, "id = ?", space).Error != nil {
		return Agent{}, "", code("not_found")
	}
	return s.createAgent(Principal{UserID: sp.Owner}, name, space, prefix, scopes, connection, days)
}

// ListAgents reports every agent's lifecycle metadata; hashes never leave
// the store.
func (s *Service) ListAgents() []Agent {
	var out []Agent
	// Agent.Expires maps to the legacy "expires" column (no column tag), so
	// order by the real column name; "expires_at" would silently return empty.
	s.DB.Order("expires").Find(&out)
	return out
}
