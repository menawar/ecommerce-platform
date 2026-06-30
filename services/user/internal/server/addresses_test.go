package server_test

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	userv1 "github.com/menawar/ecommerce-platform/proto/user/v1"
)

func sampleAddrInput() *userv1.AddressInput {
	return &userv1.AddressInput{
		Recipient: "Ada Lovelace", Phone: "08030000000",
		Line1: "1 Rayfield Rd", City: "Jos", State: "Plateau",
	}
}

// newTestClient (server_test.go) wires a real Server with in-memory stores. The
// user_id passed in the requests stands in for what the Gateway fills from the JWT.
func TestAddresses_CreateListUpdateDelete(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	const uid = "11111111-1111-1111-1111-111111111111"

	created, err := client.CreateAddress(ctx, &userv1.CreateAddressRequest{UserId: uid, Address: sampleAddrInput(), IsDefault: true})
	if err != nil {
		t.Fatalf("CreateAddress: %v", err)
	}
	addr := created.GetAddress()
	if addr.GetId() == "" || !addr.GetIsDefault() || addr.GetCountry() != "NG" {
		t.Fatalf("created address looks wrong: %+v", addr)
	}

	list, err := client.ListAddresses(ctx, &userv1.ListAddressesRequest{UserId: uid})
	if err != nil {
		t.Fatalf("ListAddresses: %v", err)
	}
	if len(list.GetAddresses()) != 1 || list.GetAddresses()[0].GetId() != addr.GetId() {
		t.Fatalf("list = %+v, want the one created", list.GetAddresses())
	}

	in := sampleAddrInput()
	in.City = "Bukuru"
	if _, err := client.UpdateAddress(ctx, &userv1.UpdateAddressRequest{UserId: uid, AddressId: addr.GetId(), Address: in}); err != nil {
		t.Fatalf("UpdateAddress: %v", err)
	}
	list, _ = client.ListAddresses(ctx, &userv1.ListAddressesRequest{UserId: uid})
	if list.GetAddresses()[0].GetCity() != "Bukuru" {
		t.Errorf("city after update = %q, want Bukuru", list.GetAddresses()[0].GetCity())
	}

	if _, err := client.DeleteAddress(ctx, &userv1.DeleteAddressRequest{UserId: uid, AddressId: addr.GetId()}); err != nil {
		t.Fatalf("DeleteAddress: %v", err)
	}
	list, _ = client.ListAddresses(ctx, &userv1.ListAddressesRequest{UserId: uid})
	if len(list.GetAddresses()) != 0 {
		t.Errorf("list after delete = %d, want 0", len(list.GetAddresses()))
	}
}

func TestAddresses_SetDefault(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	const uid = "22222222-2222-2222-2222-222222222222"

	a1, _ := client.CreateAddress(ctx, &userv1.CreateAddressRequest{UserId: uid, Address: sampleAddrInput(), IsDefault: true})
	a2, _ := client.CreateAddress(ctx, &userv1.CreateAddressRequest{UserId: uid, Address: sampleAddrInput()})

	if _, err := client.SetDefaultAddress(ctx, &userv1.SetDefaultAddressRequest{UserId: uid, AddressId: a2.GetAddress().GetId()}); err != nil {
		t.Fatalf("SetDefaultAddress: %v", err)
	}
	list, _ := client.ListAddresses(ctx, &userv1.ListAddressesRequest{UserId: uid})
	// a2 should now be default and sort first.
	if list.GetAddresses()[0].GetId() != a2.GetAddress().GetId() || !list.GetAddresses()[0].GetIsDefault() {
		t.Errorf("a2 should be the default and first; got %+v", list.GetAddresses())
	}
	for _, a := range list.GetAddresses() {
		if a.GetId() == a1.GetAddress().GetId() && a.GetIsDefault() {
			t.Error("a1 should no longer be default")
		}
	}
}

func TestAddresses_FirstIsAutoDefault(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	const uid = "55555555-5555-5555-5555-555555555555"

	// First address, default NOT requested → still becomes the default.
	created, err := client.CreateAddress(ctx, &userv1.CreateAddressRequest{UserId: uid, Address: sampleAddrInput()})
	if err != nil {
		t.Fatalf("CreateAddress: %v", err)
	}
	if !created.GetAddress().GetIsDefault() {
		t.Error("the first saved address should auto-default")
	}
	// Second one stays non-default.
	a2, _ := client.CreateAddress(ctx, &userv1.CreateAddressRequest{UserId: uid, Address: sampleAddrInput()})
	if a2.GetAddress().GetIsDefault() {
		t.Error("a later address should not auto-default")
	}
}

func TestAddresses_RejectsOverlongField(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	const uid = "66666666-6666-6666-6666-666666666666"

	in := sampleAddrInput()
	in.Line1 = strings.Repeat("x", 300)
	_, err := client.CreateAddress(ctx, &userv1.CreateAddressRequest{UserId: uid, Address: in})
	if status.Code(err) != codes.InvalidArgument {
		t.Errorf("overlong line1: want InvalidArgument, got %v", err)
	}
}

func TestAddresses_Validation(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	const uid = "33333333-3333-3333-3333-333333333333"

	t.Run("missing recipient", func(t *testing.T) {
		in := sampleAddrInput()
		in.Recipient = ""
		_, err := client.CreateAddress(ctx, &userv1.CreateAddressRequest{UserId: uid, Address: in})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("want InvalidArgument, got %v", err)
		}
	})

	t.Run("bad user_id", func(t *testing.T) {
		_, err := client.CreateAddress(ctx, &userv1.CreateAddressRequest{UserId: "not-a-uuid", Address: sampleAddrInput()})
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("want InvalidArgument, got %v", err)
		}
	})

	t.Run("unknown address → NotFound", func(t *testing.T) {
		_, err := client.UpdateAddress(ctx, &userv1.UpdateAddressRequest{
			UserId: uid, AddressId: "44444444-4444-4444-4444-444444444444", Address: sampleAddrInput(),
		})
		if status.Code(err) != codes.NotFound {
			t.Errorf("want NotFound, got %v", err)
		}
	})
}
