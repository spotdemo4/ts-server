// Code generated by BobGen sql (devel). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/maphash"
	"strings"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/clause"
	"github.com/stephenafamo/bob/dialect/sqlite"
	"github.com/stephenafamo/bob/dialect/sqlite/dialect"
	"github.com/stephenafamo/bob/orm"
	sqliteDriver "modernc.org/sqlite"
)

var TableNames = struct {
	Credentials      string
	Files            string
	Items            string
	SchemaMigrations string
	Users            string
}{
	Credentials:      "credential",
	Files:            "file",
	Items:            "item",
	SchemaMigrations: "schema_migrations",
	Users:            "user",
}

var ColumnNames = struct {
	Credentials      credentialColumnNames
	Files            fileColumnNames
	Items            itemColumnNames
	SchemaMigrations schemaMigrationColumnNames
	Users            userColumnNames
}{
	Credentials: credentialColumnNames{
		CredID:                "cred_id",
		CredPublicKey:         "cred_public_key",
		SignCount:             "sign_count",
		Transports:            "transports",
		UserVerified:          "user_verified",
		BackupEligible:        "backup_eligible",
		BackupState:           "backup_state",
		AttestationObject:     "attestation_object",
		AttestationClientData: "attestation_client_data",
		CreatedAt:             "created_at",
		LastUsed:              "last_used",
		UserID:                "user_id",
	},
	Files: fileColumnNames{
		ID:     "id",
		Name:   "name",
		Data:   "data",
		UserID: "user_id",
	},
	Items: itemColumnNames{
		ID:          "id",
		Name:        "name",
		Added:       "added",
		Description: "description",
		Price:       "price",
		Quantity:    "quantity",
		UserID:      "user_id",
	},
	SchemaMigrations: schemaMigrationColumnNames{
		Version: "version",
	},
	Users: userColumnNames{
		ID:               "id",
		Username:         "username",
		Password:         "password",
		ProfilePictureID: "profile_picture_id",
		WebauthnID:       "webauthn_id",
	},
}

var (
	SelectWhere     = Where[*dialect.SelectQuery]()
	UpdateWhere     = Where[*dialect.UpdateQuery]()
	DeleteWhere     = Where[*dialect.DeleteQuery]()
	OnConflictWhere = Where[*clause.ConflictClause]() // Used in ON CONFLICT DO UPDATE
)

func Where[Q sqlite.Filterable]() struct {
	Credentials      credentialWhere[Q]
	Files            fileWhere[Q]
	Items            itemWhere[Q]
	SchemaMigrations schemaMigrationWhere[Q]
	Users            userWhere[Q]
} {
	return struct {
		Credentials      credentialWhere[Q]
		Files            fileWhere[Q]
		Items            itemWhere[Q]
		SchemaMigrations schemaMigrationWhere[Q]
		Users            userWhere[Q]
	}{
		Credentials:      buildCredentialWhere[Q](CredentialColumns),
		Files:            buildFileWhere[Q](FileColumns),
		Items:            buildItemWhere[Q](ItemColumns),
		SchemaMigrations: buildSchemaMigrationWhere[Q](SchemaMigrationColumns),
		Users:            buildUserWhere[Q](UserColumns),
	}
}

var Preload = getPreloaders()

type preloaders struct {
	Credential credentialPreloader
	File       filePreloader
	Item       itemPreloader
	User       userPreloader
}

func getPreloaders() preloaders {
	return preloaders{
		Credential: buildCredentialPreloader(),
		File:       buildFilePreloader(),
		Item:       buildItemPreloader(),
		User:       buildUserPreloader(),
	}
}

var (
	SelectThenLoad = getThenLoaders[*dialect.SelectQuery]()
	InsertThenLoad = getThenLoaders[*dialect.InsertQuery]()
	UpdateThenLoad = getThenLoaders[*dialect.UpdateQuery]()
)

type thenLoaders[Q orm.Loadable] struct {
	Credential credentialThenLoader[Q]
	File       fileThenLoader[Q]
	Item       itemThenLoader[Q]
	User       userThenLoader[Q]
}

func getThenLoaders[Q orm.Loadable]() thenLoaders[Q] {
	return thenLoaders[Q]{
		Credential: buildCredentialThenLoader[Q](),
		File:       buildFileThenLoader[Q](),
		Item:       buildItemThenLoader[Q](),
		User:       buildUserThenLoader[Q](),
	}
}

func thenLoadBuilder[Q orm.Loadable, T any](name string, f func(context.Context, bob.Executor, T, ...bob.Mod[*dialect.SelectQuery]) error) func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q] {
	return func(queryMods ...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q] {
		return orm.Loader[Q](func(ctx context.Context, exec bob.Executor, retrieved any) error {
			loader, isLoader := retrieved.(T)
			if !isLoader {
				return fmt.Errorf("object %T cannot load %q", retrieved, name)
			}

			err := f(ctx, exec, loader, queryMods...)

			// Don't cause an issue due to missing relationships
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}

			return err
		})
	}
}

var (
	SelectJoins = getJoins[*dialect.SelectQuery]
	UpdateJoins = getJoins[*dialect.UpdateQuery]
)

type joinSet[Q interface{ aliasedAs(string) Q }] struct {
	InnerJoin Q
	LeftJoin  Q
	RightJoin Q
}

func (j joinSet[Q]) AliasedAs(alias string) joinSet[Q] {
	return joinSet[Q]{
		InnerJoin: j.InnerJoin.aliasedAs(alias),
		LeftJoin:  j.LeftJoin.aliasedAs(alias),
		RightJoin: j.RightJoin.aliasedAs(alias),
	}
}

type joins[Q dialect.Joinable] struct {
	Credentials joinSet[credentialJoins[Q]]
	Files       joinSet[fileJoins[Q]]
	Items       joinSet[itemJoins[Q]]
	Users       joinSet[userJoins[Q]]
}

func buildJoinSet[Q interface{ aliasedAs(string) Q }, C any, F func(C, string) Q](c C, f F) joinSet[Q] {
	return joinSet[Q]{
		InnerJoin: f(c, clause.InnerJoin),
		LeftJoin:  f(c, clause.LeftJoin),
		RightJoin: f(c, clause.RightJoin),
	}
}

func getJoins[Q dialect.Joinable]() joins[Q] {
	return joins[Q]{
		Credentials: buildJoinSet[credentialJoins[Q]](CredentialColumns, buildCredentialJoins),
		Files:       buildJoinSet[fileJoins[Q]](FileColumns, buildFileJoins),
		Items:       buildJoinSet[itemJoins[Q]](ItemColumns, buildItemJoins),
		Users:       buildJoinSet[userJoins[Q]](UserColumns, buildUserJoins),
	}
}

type modAs[Q any, C interface{ AliasedAs(string) C }] struct {
	c C
	f func(C) bob.Mod[Q]
}

func (m modAs[Q, C]) Apply(q Q) {
	m.f(m.c).Apply(q)
}

func (m modAs[Q, C]) AliasedAs(alias string) bob.Mod[Q] {
	m.c = m.c.AliasedAs(alias)
	return m
}

func randInt() int64 {
	out := int64(new(maphash.Hash).Sum64())

	if out < 0 {
		return -out % 10000
	}

	return out % 10000
}

// ErrUniqueConstraint captures all unique constraint errors by explicitly leaving `s` empty.
var ErrUniqueConstraint = &UniqueConstraintError{s: ""}

type UniqueConstraintError struct {
	// schema is the schema where the unique constraint is defined.
	schema string
	// table is the name of the table where the unique constraint is defined.
	table string
	// columns are the columns constituting the unique constraint.
	columns []string
	// s is a string uniquely identifying the constraint in the raw error message returned from the database.
	s string
}

func (e *UniqueConstraintError) Error() string {
	return e.s
}

func (e *UniqueConstraintError) Is(target error) bool {
	var err *sqliteDriver.Error
	if !errors.As(target, &err) {
		return false
	}

	// 1555 is for Primary Key Constraint
	// 2067 is for Unique Constraint
	if err.Code() != 1555 && err.Code() != 2067 {
		return false
	}

	for _, col := range e.columns {
		if !strings.Contains(err.Error(), fmt.Sprintf("%s.%s", e.table, col)) {
			return false
		}
	}

	return true
}
