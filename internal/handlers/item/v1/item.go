package item

import (
	"context"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/aarondl/opt/omit"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/sqlite/sm"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/spotdemo4/ts-server/internal/app"
	"github.com/spotdemo4/ts-server/internal/auth"
	itemv1 "github.com/spotdemo4/ts-server/internal/connect/item/v1"
	"github.com/spotdemo4/ts-server/internal/connect/item/v1/itemv1connect"
	"github.com/spotdemo4/ts-server/internal/models"
	"github.com/spotdemo4/ts-server/internal/putil"
)

type Handler struct {
	db   *bob.DB
	auth *auth.Auth
}

// GetItem retrieves an item by its ID.
func (h *Handler) GetItem(
	ctx context.Context,
	req *connect.Request[itemv1.GetItemRequest],
) (*connect.Response[itemv1.GetItemResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	item, err := models.Items.Query(
		models.SelectWhere.Items.ID.EQ(req.Msg.GetId()),
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

const DefaultLimit = 10

// GetItems retrieves a list of items for a user.
func (h *Handler) GetItems(
	ctx context.Context,
	req *connect.Request[itemv1.GetItemsRequest],
) (*connect.Response[itemv1.GetItemsResponse], error) {
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
			models.SelectWhere.Items.Name.Like(req.Msg.GetFilter()),
		)
	}

	// Start
	if req.Msg.GetStart() != nil {
		query.Apply(
			models.SelectWhere.Items.Added.GTE(req.Msg.GetStart().AsTime()),
		)
	}

	// End
	if req.Msg.GetEnd() != nil {
		query.Apply(
			models.SelectWhere.Items.Added.LTE(req.Msg.GetEnd().AsTime()),
		)
	}

	// Count
	count, err := query.Count(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Limit
	if req.Msg.Limit != nil {
		query.Apply(sm.Limit(req.Msg.GetLimit()))
	} else {
		query.Apply(sm.Limit(DefaultLimit))
	}

	// Offset
	if req.Msg.Offset != nil {
		query.Apply(sm.Offset(req.Msg.GetOffset()))
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

// CreateItem creates a new item for a user.
func (h *Handler) CreateItem(
	ctx context.Context,
	req *connect.Request[itemv1.CreateItemRequest],
) (*connect.Response[itemv1.CreateItemResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	item, err := models.Items.Insert(
		&models.ItemSetter{
			Name:        omit.From(req.Msg.GetName()),
			Added:       omit.From(time.Now()),
			Description: omit.From(req.Msg.GetDescription()),
			Price:       omit.From(req.Msg.GetPrice()),
			Quantity:    omit.From(req.Msg.GetQuantity()),
			UserID:      omit.From(user.ID),
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

// UpdateItem updates an existing item.
func (h *Handler) UpdateItem(
	ctx context.Context,
	req *connect.Request[itemv1.UpdateItemRequest],
) (*connect.Response[itemv1.UpdateItemResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Get item
	item, err := models.Items.Query(
		models.SelectWhere.Items.ID.EQ(req.Msg.GetId()),
		models.SelectWhere.Items.UserID.EQ(user.ID),
	).One(ctx, h.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	// Update item
	err = item.Update(ctx, h.db, &models.ItemSetter{
		Name:        omit.From(req.Msg.GetName()),
		Description: omit.From(req.Msg.GetDescription()),
		Price:       omit.From(req.Msg.GetPrice()),
		Quantity:    omit.From(req.Msg.GetQuantity()),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	res := connect.NewResponse(&itemv1.UpdateItemResponse{})
	return res, nil
}

// DeleteItem deletes a user's item.
func (h *Handler) DeleteItem(
	ctx context.Context,
	req *connect.Request[itemv1.DeleteItemRequest],
) (*connect.Response[itemv1.DeleteItemResponse], error) {
	user, ok := h.auth.GetContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Get item
	item, err := models.Items.Query(
		models.SelectWhere.Items.ID.EQ(req.Msg.GetId()),
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

// New creates a new Item service handler.
func New(app *app.App, interceptors connect.Option) (string, http.Handler) {
	return itemv1connect.NewItemServiceHandler(
		&Handler{
			db:   app.DB,
			auth: app.Auth,
		},
		interceptors,
	)
}
