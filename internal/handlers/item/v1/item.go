package item

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	itemv1 "github.com/spotdemo4/ts-server/internal/connect/item/v1"
	"github.com/spotdemo4/ts-server/internal/connect/item/v1/itemv1connect"
	"github.com/spotdemo4/ts-server/internal/interceptors"
	"github.com/spotdemo4/ts-server/internal/putil"
	"github.com/spotdemo4/ts-server/internal/sqlc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func itemToConnect(item sqlc.Item) *itemv1.Item {
	timestamp := timestamppb.New(item.Added)

	return &itemv1.Item{
		Id:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Price:       float32(item.Price),
		Quantity:    int32(item.Quantity),
		Added:       timestamp,
	}
}

type Handler struct {
	db  *sqlc.Queries
	key []byte
}

func (h *Handler) GetItem(ctx context.Context, req *connect.Request[itemv1.GetItemRequest]) (*connect.Response[itemv1.GetItemResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Get item
	item, err := h.db.GetItem(ctx, sqlc.GetItemParams{
		ID:     req.Msg.Id,
		UserID: userid,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&itemv1.GetItemResponse{
		Item: itemToConnect(item),
	})
	return res, nil
}

func (h *Handler) GetItems(ctx context.Context, req *connect.Request[itemv1.GetItemsRequest]) (*connect.Response[itemv1.GetItemsResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Verify
	offset := 0
	if req.Msg.Offset != nil {
		offset = int(*req.Msg.Offset)
	}

	limit := 10
	if req.Msg.Limit != nil {
		limit = int(*req.Msg.Limit)
	}

	// Get items
	items, err := h.db.GetItems(ctx, sqlc.GetItemsParams{
		UserID: userid,
		Name:   putil.NullLike(req.Msg.Filter),
		Start:  putil.NullTimestamp(req.Msg.Start),
		End:    putil.NullTimestamp(req.Msg.End),
		Offset: int64(offset),
		Limit:  int64(limit),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}

		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get items count
	count, err := h.db.GetItemsCount(ctx, sqlc.GetItemsCountParams{
		UserID: userid,
		Name:   putil.NullLike(req.Msg.Filter),
		Start:  putil.NullTimestamp(req.Msg.Start),
		End:    putil.NullTimestamp(req.Msg.End),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert to connect items
	resItems := []*itemv1.Item{}
	for _, item := range items {
		resItems = append(resItems, itemToConnect(item))
	}

	res := connect.NewResponse(&itemv1.GetItemsResponse{
		Items: resItems,
		Count: count,
	})
	return res, nil
}

func (h *Handler) CreateItem(ctx context.Context, req *connect.Request[itemv1.CreateItemRequest]) (*connect.Response[itemv1.CreateItemResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	time := time.Now()

	// Insert item
	id, err := h.db.InsertItem(ctx, sqlc.InsertItemParams{
		Name:        req.Msg.Name,
		Added:       time,
		Description: req.Msg.Description,
		Price:       float64(req.Msg.Price),
		Quantity:    int64(req.Msg.Quantity),
		UserID:      userid,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&itemv1.CreateItemResponse{
		Id:    id,
		Added: timestamppb.New(time),
	})
	return res, nil
}

func (h *Handler) UpdateItem(ctx context.Context, req *connect.Request[itemv1.UpdateItemRequest]) (*connect.Response[itemv1.UpdateItemResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Update item
	err := h.db.UpdateItem(ctx, sqlc.UpdateItemParams{
		// set
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		Price:       putil.NullFloat64(req.Msg.Price),
		Quantity:    putil.NullInt64(req.Msg.Quantity),

		// where
		ID:     req.Msg.Id,
		UserID: userid,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&itemv1.UpdateItemResponse{})
	return res, nil
}

func (h *Handler) DeleteItem(ctx context.Context, req *connect.Request[itemv1.DeleteItemRequest]) (*connect.Response[itemv1.DeleteItemResponse], error) {
	userid, ok := interceptors.GetUserContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}

	// Delete item
	err := h.db.DeleteItem(ctx, sqlc.DeleteItemParams{
		ID:     req.Msg.Id,
		UserID: userid,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&itemv1.DeleteItemResponse{})
	return res, nil
}

func NewHandler(vi *validate.Interceptor, db *sqlc.Queries, key string) (string, http.Handler) {
	interceptors := connect.WithInterceptors(vi, interceptors.NewAuthInterceptor(key))

	return itemv1connect.NewItemServiceHandler(
		&Handler{
			db:  db,
			key: []byte(key),
		},
		interceptors,
	)
}
