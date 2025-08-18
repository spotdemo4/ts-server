package item

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/spotdemo4/ts-server/internal/bob/models"
	itemv1 "github.com/spotdemo4/ts-server/internal/connect/item/v1"
)

func itemToConnect(item models.Item) *itemv1.Item {
	return &itemv1.Item{
		Id:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		Price:       item.Price,
		Quantity:    item.Quantity,
		Added:       timestamppb.New(item.Added),
	}
}
