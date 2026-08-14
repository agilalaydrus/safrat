package handler

import (
	"context"

	"connectrpc.com/connect"
	hajjv1 "github.com/hajj-saas/api/internal/gen/hajj/v1"
	"github.com/hajj-saas/api/internal/middleware"
	"github.com/hajj-saas/api/internal/service"
)

type ProductHandler struct{ productService *service.ProductService }

func NewProductHandler(productService *service.ProductService) *ProductHandler {
	return &ProductHandler{productService: productService}
}
func (h *ProductHandler) CreateProduct(ctx context.Context, req *connect.Request[hajjv1.CreateProductRequest]) (*connect.Response[hajjv1.Product], error) {
	result, err := h.productService.Create(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *ProductHandler) GetProduct(ctx context.Context, req *connect.Request[hajjv1.GetProductRequest]) (*connect.Response[hajjv1.Product], error) {
	result, err := h.productService.Get(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *ProductHandler) ListProducts(ctx context.Context, req *connect.Request[hajjv1.ListProductsRequest]) (*connect.Response[hajjv1.ListProductsResponse], error) {
	result, err := h.productService.List(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *ProductHandler) UpdateProduct(ctx context.Context, req *connect.Request[hajjv1.UpdateProductRequest]) (*connect.Response[hajjv1.Product], error) {
	result, err := h.productService.Update(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
func (h *ProductHandler) DeleteProduct(ctx context.Context, req *connect.Request[hajjv1.DeleteProductRequest]) (*connect.Response[hajjv1.DeleteProductResponse], error) {
	result, err := h.productService.Delete(ctx, middleware.OperatorIDFromCtx(ctx), req.Msg)
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(result), nil
}
