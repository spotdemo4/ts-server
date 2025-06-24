package item

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/spotdemo4/ts-server/internal/auth"
	itemv1 "github.com/spotdemo4/ts-server/internal/connect/item/v1"
	"github.com/spotdemo4/ts-server/internal/connect/item/v1/itemv1connect"
	"github.com/spotdemo4/ts-server/internal/models"
	"github.com/spotdemo4/ts-server/internal/putil"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/sqlite/sm"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func itemToConnect(item models.Item) *itemv1.Item {
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
	db   *bob.DB
	auth *auth.Auth
}

func (h *Handler) GetItem(ctx context.Context, req *connect.Request[itemv1.GetItemRequest]) (*connect.Response[itemv1.GetItemResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	item, err := models.Items.Query(
		models.SelectWhere.Items.ID.EQ(int32(req.Msg.Id)),
		models.SelectWhere.Items.UserID.EQ(user.ID),
	).One(ctx, h.db)
	if err != nil {
		return nil, putil.CheckNotFound(err)
	}

	res := connect.NewResponse(&itemv1.GetItemResponse{
		Item: itemToConnect(*item),
	})
	return res, nil
}

func (h *Handler) GetItems(ctx context.Context, req *connect.Request[itemv1.GetItemsRequest]) (*connect.Response[itemv1.GetItemsResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	query := models.Items.Query(
		models.SelectWhere.Items.UserID.EQ(user.ID),
	)

	// Filter
	if req.Msg.Filter != nil {
		query.Apply(
			models.SelectWhere.Items.Name.Like(*req.Msg.Filter),
		)
	}

	// Start
	if req.Msg.Start != nil {
		query.Apply(
			models.SelectWhere.Items.Added.GTE(req.Msg.Start.AsTime()),
		)
	}

	// End
	if req.Msg.End != nil {
		query.Apply(
			models.SelectWhere.Items.Added.LTE(req.Msg.End.AsTime()),
		)
	}

	// Count
	count, err := query.Count(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Limit
	if req.Msg.Limit != nil {
		query.Apply(sm.Limit(*req.Msg.Limit))
	} else {
		query.Apply(sm.Limit(10))
	}

	// Offset
	if req.Msg.Offset != nil {
		query.Apply(sm.Offset(*req.Msg.Offset))
	} else {
		query.Apply(sm.Offset(0))
	}

	// Items
	items, err := query.All(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Convert to connect items
	resItems := []*itemv1.Item{}
	for _, item := range items {
		if item != nil {
			resItems = append(resItems, itemToConnect(*item))
		}
	}

	res := connect.NewResponse(&itemv1.GetItemsResponse{
		Items: resItems,
		Count: count,
	})
	return res, nil
}

func (h *Handler) CreateItem(ctx context.Context, req *connect.Request[itemv1.CreateItemRequest]) (*connect.Response[itemv1.CreateItemResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	item, err := models.Items.Insert(
		&models.ItemSetter{
			Name:        &req.Msg.Name,
			Added:       putil.ToPointer(time.Now()),
			Description: &req.Msg.Description,
			Price:       &req.Msg.Price,
			Quantity:    &req.Msg.Quantity,
			UserID:      &user.ID,
		},
	).One(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&itemv1.CreateItemResponse{
		Id:    item.ID,
		Added: timestamppb.New(item.Added),
	})
	return res, nil
}

func (h *Handler) UpdateItem(ctx context.Context, req *connect.Request[itemv1.UpdateItemRequest]) (*connect.Response[itemv1.UpdateItemResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Get item
	item, err := models.Items.Query(
		models.SelectWhere.Items.ID.EQ(req.Msg.Id),
		models.SelectWhere.Items.UserID.EQ(user.ID),
	).One(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	// Update item
	err = item.Update(ctx, h.db, &models.ItemSetter{
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		Price:       req.Msg.Price,
		Quantity:    req.Msg.Quantity,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&itemv1.UpdateItemResponse{})
	return res, nil
}

func (h *Handler) DeleteItem(ctx context.Context, req *connect.Request[itemv1.DeleteItemRequest]) (*connect.Response[itemv1.DeleteItemResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Get item
	item, err := models.Items.Query(
		models.SelectWhere.Items.ID.EQ(req.Msg.Id),
		models.SelectWhere.Items.UserID.EQ(user.ID),
	).One(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	// Delete item
	err = item.Delete(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&itemv1.DeleteItemResponse{})
	return res, nil
}

func NewHandler(db *bob.DB, auth *auth.Auth, interceptors connect.Option) (string, http.Handler) {
	return itemv1connect.NewItemServiceHandler(
		&Handler{
			db:   db,
			auth: auth,
		},
		interceptors,
	)
}
