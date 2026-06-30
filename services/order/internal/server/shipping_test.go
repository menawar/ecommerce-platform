package server_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	orderv1 "github.com/menawar/ecommerce-platform/proto/order/v1"
)

func shippingInput(name string, price int64, active bool) *orderv1.ShippingMethodInput {
	return &orderv1.ShippingMethodInput{Name: name, Description: "desc", PriceCents: price, SortOrder: 1, Active: active}
}

func TestShippingMethods_CRUDAndActiveFilter(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if _, err := pool.Exec(ctx, "TRUNCATE shipping_methods"); err != nil {
		t.Fatalf("truncate shipping_methods: %v", err)
	}
	client := newClient(t, pool)

	std, err := client.CreateShippingMethod(ctx, &orderv1.CreateShippingMethodRequest{Method: shippingInput("Standard", 150000, true)})
	if err != nil {
		t.Fatalf("CreateShippingMethod: %v", err)
	}
	if std.GetMethod().GetId() == "" || std.GetMethod().GetPriceCents() != 150000 {
		t.Fatalf("created method looks wrong: %+v", std.GetMethod())
	}
	// A disabled method.
	if _, err := client.CreateShippingMethod(ctx, &orderv1.CreateShippingMethodRequest{Method: shippingInput("Pickup", 0, false)}); err != nil {
		t.Fatalf("Create disabled: %v", err)
	}

	t.Run("active_only filters; full list includes disabled", func(t *testing.T) {
		active, _ := client.ListShippingMethods(ctx, &orderv1.ListShippingMethodsRequest{ActiveOnly: true})
		if len(active.GetMethods()) != 1 || active.GetMethods()[0].GetName() != "Standard" {
			t.Errorf("active list = %+v, want only Standard", active.GetMethods())
		}
		all, _ := client.ListShippingMethods(ctx, &orderv1.ListShippingMethodsRequest{ActiveOnly: false})
		if len(all.GetMethods()) != 2 {
			t.Errorf("full list = %d, want 2", len(all.GetMethods()))
		}
	})

	t.Run("update", func(t *testing.T) {
		upd, err := client.UpdateShippingMethod(ctx, &orderv1.UpdateShippingMethodRequest{
			Id: std.GetMethod().GetId(), Method: shippingInput("Standard", 200000, true),
		})
		if err != nil {
			t.Fatalf("UpdateShippingMethod: %v", err)
		}
		if upd.GetMethod().GetPriceCents() != 200000 {
			t.Errorf("price after update = %d, want 200000", upd.GetMethod().GetPriceCents())
		}
	})

	t.Run("delete", func(t *testing.T) {
		if _, err := client.DeleteShippingMethod(ctx, &orderv1.DeleteShippingMethodRequest{Id: std.GetMethod().GetId()}); err != nil {
			t.Fatalf("DeleteShippingMethod: %v", err)
		}
		all, _ := client.ListShippingMethods(ctx, &orderv1.ListShippingMethodsRequest{ActiveOnly: false})
		if len(all.GetMethods()) != 1 {
			t.Errorf("after delete = %d, want 1", len(all.GetMethods()))
		}
	})
}

func TestShippingMethods_Validation(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	if _, err := pool.Exec(ctx, "TRUNCATE shipping_methods"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	client := newClient(t, pool)

	t.Run("missing name", func(t *testing.T) {
		_, err := client.CreateShippingMethod(ctx, &orderv1.CreateShippingMethodRequest{Method: shippingInput("", 1000, true)})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("want InvalidArgument, got %v", err)
		}
	})
	t.Run("negative price", func(t *testing.T) {
		_, err := client.CreateShippingMethod(ctx, &orderv1.CreateShippingMethodRequest{Method: shippingInput("Bad", -1, true)})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("want InvalidArgument, got %v", err)
		}
	})
	t.Run("update unknown → NotFound", func(t *testing.T) {
		_, err := client.UpdateShippingMethod(ctx, &orderv1.UpdateShippingMethodRequest{
			Id: "00000000-0000-0000-0000-000000000000", Method: shippingInput("X", 1000, true),
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("want NotFound, got %v", err)
		}
	})
	t.Run("delete unknown → NotFound", func(t *testing.T) {
		_, err := client.DeleteShippingMethod(ctx, &orderv1.DeleteShippingMethodRequest{Id: "00000000-0000-0000-0000-000000000000"})
		if status.Code(err) != codes.NotFound {
			t.Errorf("want NotFound, got %v", err)
		}
	})
}
