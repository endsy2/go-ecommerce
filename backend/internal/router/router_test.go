package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"ecommerce/backend/internal/config"
	"ecommerce/backend/internal/domain"
	"ecommerce/backend/internal/handler"
	"ecommerce/backend/internal/model"
	"ecommerce/backend/internal/service"
	"ecommerce/backend/internal/util"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// These tests are about the ROUTE TABLE, not about business logic: which
// middleware a route sits behind, and therefore who is allowed to reach it. The
// services below are stubs precisely so that a failure here can only mean the
// wiring is wrong.

const (
	adminToken    = "admin.token"
	customerToken = "customer.token"
)

// fakeTokens maps two fixed strings onto two roles and rejects everything else.
type fakeTokens struct{}

func (fakeTokens) ValidateToken(raw string) (*util.Claims, error) {
	role := model.RoleCustomer
	switch raw {
	case adminToken:
		role = model.RoleAdmin
	case customerToken:
		role = model.RoleCustomer
	default:
		return nil, util.ErrInvalidToken
	}

	return &util.Claims{
		Role:             role,
		RegisteredClaims: jwt.RegisteredClaims{Subject: uuid.New().String()},
	}, nil
}

type fakeAuthService struct{}

func (fakeAuthService) Register(context.Context, string, string, string) (*model.User, string, error) {
	return testUser(), "token", nil
}
func (fakeAuthService) Login(context.Context, string, string) (string, error) { return "token", nil }
func (fakeAuthService) CurrentUser(context.Context, uuid.UUID) (*model.User, error) {
	return testUser(), nil
}

func testUser() *model.User {
	u := &model.User{Email: "bob@example.com", Name: "Bob", Role: model.RoleCustomer}
	u.ID = uuid.New()
	return u
}

type fakeProductService struct{}

func (fakeProductService) List(_ context.Context, limit, _ int) ([]model.Product, int64, error) {
	return []model.Product{}, 0, nil
}
func (fakeProductService) GetBySlug(_ context.Context, slug string) (*model.Product, error) {
	if slug != "blue-mug" {
		return nil, domain.NotFoundf("product not found")
	}
	p := &model.Product{Name: "Blue Mug", Slug: slug, PriceCents: 1999}
	p.ID = uuid.New()
	return p, nil
}
func (fakeProductService) Create(context.Context, service.ProductInput) (*model.Product, error) {
	p := &model.Product{Name: "Blue Mug", Slug: "blue-mug"}
	p.ID = uuid.New()
	return p, nil
}
func (fakeProductService) Update(context.Context, uuid.UUID, service.ProductInput) (*model.Product, error) {
	p := &model.Product{Name: "Blue Mug", Slug: "blue-mug"}
	p.ID = uuid.New()
	return p, nil
}
func (fakeProductService) Delete(context.Context, uuid.UUID) error { return nil }

type fakeCategoryService struct{}

func (fakeCategoryService) List(context.Context, int, int) ([]model.Category, int64, error) {
	return []model.Category{}, 0, nil
}
func (fakeCategoryService) GetBySlug(_ context.Context, slug string) (*model.Category, error) {
	c := &model.Category{Name: "Kitchen", Slug: slug}
	c.ID = uuid.New()
	return c, nil
}
func (fakeCategoryService) Create(context.Context, string, string) (*model.Category, error) {
	c := &model.Category{Name: "Kitchen", Slug: "kitchen"}
	c.ID = uuid.New()
	return c, nil
}
func (fakeCategoryService) Update(context.Context, uuid.UUID, string) (*model.Category, error) {
	c := &model.Category{Name: "Kitchen", Slug: "kitchen"}
	c.ID = uuid.New()
	return c, nil
}
func (fakeCategoryService) Delete(context.Context, uuid.UUID) error { return nil }

func testEngine() *gin.Engine {
	cfg := &config.Config{Env: "development"}

	return New(cfg, Handlers{
		// nil connections are safe: only /health/ready touches them, and no
		// test below calls it.
		Health:   handler.NewHealthHandler(nil, nil),
		Auth:     handler.NewAuthHandler(fakeAuthService{}, time.Hour, false),
		Product:  handler.NewProductHandler(fakeProductService{}),
		Category: handler.NewCategoryHandler(fakeCategoryService{}),
	}, fakeTokens{})
}

func do(t *testing.T, engine *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader = strings.NewReader(body)
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func TestRouteAccess(t *testing.T) {
	engine := testEngine()

	productBody := `{"category_id":"` + uuid.New().String() + `","name":"Blue Mug","price_cents":1999,"stock":3}`
	someID := uuid.New().String()

	tests := []struct {
		name   string
		method string
		path   string
		token  string
		body   string
		want   int
	}{
		{
			name: "liveness needs no token", method: http.MethodGet,
			path: "/health", want: http.StatusOK,
		},
		{
			// The storefront has to render before anyone signs in.
			name: "the product list is public", method: http.MethodGet,
			path: "/api/v1/products", want: http.StatusOK,
		},
		{
			name: "a product detail page is public", method: http.MethodGet,
			path: "/api/v1/products/blue-mug", want: http.StatusOK,
		},
		{
			name: "the category list is public", method: http.MethodGet,
			path: "/api/v1/categories", want: http.StatusOK,
		},
		{
			name: "a category detail page is public", method: http.MethodGet,
			path: "/api/v1/categories/kitchen", want: http.StatusOK,
		},
		{
			name: "auth/me without a token is rejected", method: http.MethodGet,
			path: "/api/v1/auth/me", want: http.StatusUnauthorized,
		},
		{
			name: "auth/me with an invalid token is rejected", method: http.MethodGet,
			path: "/api/v1/auth/me", token: "forged", want: http.StatusUnauthorized,
		},
		{
			name: "auth/me with a valid token is admitted", method: http.MethodGet,
			path: "/api/v1/auth/me", token: customerToken, want: http.StatusOK,
		},
		{
			// 401 rather than 403: the caller has not identified themselves at
			// all, so there is no role to refuse.
			name: "creating a product anonymously is unauthorized", method: http.MethodPost,
			path: "/api/v1/products", body: productBody, want: http.StatusUnauthorized,
		},
		{
			name: "a customer cannot create a product", method: http.MethodPost,
			path: "/api/v1/products", token: customerToken, body: productBody,
			want: http.StatusForbidden,
		},
		{
			name: "an admin can create a product", method: http.MethodPost,
			path: "/api/v1/products", token: adminToken, body: productBody,
			want: http.StatusCreated,
		},
		{
			name: "a customer cannot delete a product", method: http.MethodDelete,
			path: "/api/v1/products/" + someID, token: customerToken,
			want: http.StatusForbidden,
		},
		{
			name: "an admin can delete a product", method: http.MethodDelete,
			path: "/api/v1/products/" + someID, token: adminToken,
			want: http.StatusNoContent,
		},
		{
			name: "a customer cannot rename a category", method: http.MethodPut,
			path: "/api/v1/categories/" + someID, token: customerToken,
			body: `{"name":"Kitchen"}`, want: http.StatusForbidden,
		},
		{
			name: "an admin can rename a category", method: http.MethodPut,
			path: "/api/v1/categories/" + someID, token: adminToken,
			body: `{"name":"Kitchen"}`, want: http.StatusOK,
		},
		{
			// A missing price is caught by the binding tag, not by the service.
			name: "a product body without a price is rejected", method: http.MethodPost,
			path: "/api/v1/products", token: adminToken,
			body: `{"category_id":"` + uuid.New().String() + `","name":"Blue Mug","stock":3}`,
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "a malformed uuid in the path is rejected", method: http.MethodDelete,
			path: "/api/v1/products/not-a-uuid", token: adminToken,
			want: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := do(t, engine, tt.method, tt.path, tt.token, tt.body)

			if res.Code != tt.want {
				t.Fatalf("got %d, want %d — body: %s", res.Code, tt.want, res.Body.String())
			}
		})
	}
}

// TestListLimitIsCapped is the unbounded-query guard: a client must not be able
// to ask for the whole table.
func TestListLimitIsCapped(t *testing.T) {
	engine := testEngine()

	res := do(t, engine, http.MethodGet, "/api/v1/products?limit=500", "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 — an oversized limit is clamped, not rejected", res.Code)
	}

	var body struct {
		Meta handler.Meta `json:"meta"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if body.Meta.Limit != handler.MaxLimit {
		t.Errorf("got meta.limit %d, want %d", body.Meta.Limit, handler.MaxLimit)
	}
}

// TestBadPaginationIsRejected covers the other half: garbage is a 422, unlike an
// oversized limit which is simply clamped.
func TestBadPaginationIsRejected(t *testing.T) {
	engine := testEngine()

	for _, query := range []string{"?page=abc", "?page=0", "?limit=-1"} {
		t.Run(query, func(t *testing.T) {
			res := do(t, engine, http.MethodGet, "/api/v1/products"+query, "", "")
			if res.Code != http.StatusUnprocessableEntity {
				t.Errorf("got %d, want 422 — body: %s", res.Code, res.Body.String())
			}
		})
	}
}
