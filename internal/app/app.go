package app

import (
	"context"
	"fmt"
	"laundry-app-with-golang/internal/attendance"
	"laundry-app-with-golang/internal/clothingtype"
	"laundry-app-with-golang/internal/config"
	"laundry-app-with-golang/internal/cron"
	"laundry-app-with-golang/internal/customer"
	"laundry-app-with-golang/internal/database"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/email"
	"laundry-app-with-golang/internal/employee"
	"laundry-app-with-golang/internal/geocode"
	"laundry-app-with-golang/internal/laundryitem"
	"laundry-app-with-golang/internal/notification"
	oauthpkg "laundry-app-with-golang/internal/oauth"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/outlet"
	"laundry-app-with-golang/internal/payment"
	"laundry-app-with-golang/internal/report"
	"laundry-app-with-golang/internal/server"
	"laundry-app-with-golang/internal/shift"
	"laundry-app-with-golang/internal/sse"
	"laundry-app-with-golang/internal/storage"
	"laundry-app-with-golang/internal/wilayah"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/midtrans/midtrans-go"
)

// New wires up every handler/client/dependency and returns the assembled
// router along with the DB pool. The caller owns pool.Close() and
// router.Run().
func New(cfg config.Config) (*gin.Engine, *pgxpool.Pool, error) {
	midtrans.ServerKey = cfg.MidtransServerKey
	midtrans.ClientKey = cfg.MidtransClientKey
	if cfg.MidtransIsProduction {
		midtrans.Environment = midtrans.Production
	} else {
		midtrans.Environment = midtrans.Sandbox
	}

	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	queries := db.New(pool)
	emailClient := email.NewClient(cfg.ResendAPIKey, cfg.AppBaseURL, cfg.FrontendURL)

	storageClient, err := storage.NewClient(cfg.CloudinaryURL)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("failed to init cloudinary client: %w", err)
	}

	googleClient := oauthpkg.NewGoogleClient(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.AppBaseURL)
	geocodeClient := geocode.NewClient(cfg.OpenCageAPIKey)

	customerHandler := customer.NewHandler(queries, pool, cfg, emailClient, storageClient, googleClient, geocodeClient)
	employeeHandler := employee.NewHandler(queries, pool, cfg, emailClient, geocodeClient)
	wilayahHandler := wilayah.NewHandler(queries)
	outletHandler := outlet.NewHandler(queries)
	orderHandler := order.NewHandler(pool, queries)
	laundryItemHandler := laundryitem.NewHandler(queries)
	clothingTypeHandler := clothingtype.NewHandler(queries)
	shiftHandler := shift.NewHandler(pool, queries)
	attendanceHandler := attendance.NewHandler(pool, queries, cfg)
	paymentHandler := payment.NewHandler(pool, queries, cfg)
	notificationHandler := notification.NewHandler(pool, queries)
	cronHandler := cron.NewHandler(pool, queries)
	reportHandler := report.NewHandler(pool, queries)
	sseHandler := sse.NewHandler(queries, cfg)

	cron.Start(ctx, pool, queries)

	router := server.NewRouter(customerHandler, employeeHandler, wilayahHandler, outletHandler, orderHandler, laundryItemHandler, clothingTypeHandler, shiftHandler, attendanceHandler, paymentHandler, notificationHandler, cronHandler, reportHandler, sseHandler, cfg, queries)

	return router, pool, nil
}
