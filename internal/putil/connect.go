package putil

import (
	"database/sql"
	"errors"

	"connectrpc.com/connect"
)

func CheckNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewError(connect.CodeInternal, err)
}
