package helpers

func StringSliceDiff(rolesA, rolesB []string) (added, removed []string) {
	added = make([]string, 0)
	removed = make([]string, 0)

	if rolesA == nil && rolesB == nil {
		return
	}

	if rolesA == nil {
		return rolesB, removed
	}

	if rolesB == nil {
		return added, rolesA
	}

	for _, oldRole := range rolesA {
		still := false
		for _, newRole := range rolesB {
			if oldRole == newRole {
				still = true
			}
		}
		if !still {
			removed = append(removed, oldRole)
		}
	}

	for _, newRole := range rolesB {
		new := true
		for _, oldRole := range rolesA {
			if oldRole == newRole {
				new = false
			}
		}
		if new {
			added = append(added, newRole)
		}
	}

	return
}
