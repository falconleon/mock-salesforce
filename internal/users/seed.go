package users

// SeedFromMap inserts each (username, password) pair from the given
// map into the store, but only when no user with that username already
// exists. Used at startup to lift the legacy MOCK_USERS env var into
// the runtime-mutable store without clobbering CRUD edits made in a
// previous run (relevant when the store is SQLite-backed).
func SeedFromMap(s Store, users map[string]string) error {
	for username, password := range users {
		if _, err := s.GetUserByUsername(username); err == nil {
			continue
		}
		if _, err := s.CreateUser(username, "", username, password); err != nil {
			return err
		}
	}
	return nil
}
