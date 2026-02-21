package policies

import "github.com/PauloHFS/goth/internal/db"

func CanUpdateUser(actor, target db.User) bool {
	if actor.RoleID == "admin" {
		return true
	}
	return actor.ID == target.ID
}

func CanDeleteUser(actor, target db.User) bool {
	if actor.RoleID == "admin" {
		return true
	}
	return actor.ID == target.ID
}

func CanViewUser(actor, target db.User) bool {
	if actor.RoleID == "admin" {
		return true
	}
	return actor.TenantID == target.TenantID
}
