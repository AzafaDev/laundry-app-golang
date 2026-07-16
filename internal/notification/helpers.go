package notification

import (
	"context"
	"errors"
	db "laundry-app-with-golang/internal/db/generated"
	"laundry-app-with-golang/internal/sse"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

var errNoSession = errors.New("something went wrong")

func parsePagination(c *gin.Context) (limit, offset int32) {
	limit = defaultPageLimit
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = int32(v)
		if limit > maxPageLimit {
			limit = maxPageLimit
		}
	}

	offset = 0
	if v, err := strconv.Atoi(c.Query("offset")); err == nil && v > 0 {
		offset = int32(v)
	}

	return limit, offset
}

func currentCustomerID(c *gin.Context) (pgtype.UUID, error) {
	var id pgtype.UUID
	val, ok := c.Get("customer_id")
	if !ok {
		return id, errNoSession
	}
	str, ok := val.(string)
	if !ok {
		return id, errNoSession
	}
	if err := id.Scan(str); err != nil {
		return id, err
	}
	return id, nil
}

func currentEmployeeID(c *gin.Context) (pgtype.UUID, error) {
	var id pgtype.UUID
	val, ok := c.Get("employee_id")
	if !ok {
		return id, errNoSession
	}
	str, ok := val.(string)
	if !ok {
		return id, errNoSession
	}
	if err := id.Scan(str); err != nil {
		return id, err
	}
	return id, nil
}

// NotifyCustomer creates a single in-app notification for a customer. qtx
// may be transaction-scoped or the package-level *db.Queries — the caller
// decides whether this notification is part of a larger atomic unit of
// work or a best-effort side effect after commit.
func NotifyCustomer(ctx context.Context, qtx *db.Queries, customerID pgtype.UUID, title, body, notifType string, relatedEntityID pgtype.UUID) error {
	created, err := qtx.CreateCustomerNotification(ctx, db.CreateCustomerNotificationParams{
		CustomerID:      customerID,
		Title:           title,
		Body:            body,
		Type:            notifType,
		RelatedEntityID: relatedEntityID,
	})
	if err != nil {
		return err
	}
	sse.Default.Broadcast("user:"+customerID.String(), "notification:new", created)
	return nil
}

// NotifyEmployee creates a single in-app notification for an employee.
func NotifyEmployee(ctx context.Context, qtx *db.Queries, employeeID pgtype.UUID, title, body, notifType string, relatedEntityID pgtype.UUID) error {
	created, err := qtx.CreateEmployeeNotification(ctx, db.CreateEmployeeNotificationParams{
		EmployeeID:      employeeID,
		Title:           title,
		Body:            body,
		Type:            notifType,
		RelatedEntityID: relatedEntityID,
	})
	if err != nil {
		return err
	}
	sse.Default.Broadcast("user:"+employeeID.String(), "notification:new", created)
	return nil
}

// NotifyOutletEmployees notifies every active employee at outletID whose
// role is in roles — e.g. []string{"outlet_admin", "driver"} for a
// payment-confirmed event. Mirrors the TS source's notifyOutletEmployees.
func NotifyOutletEmployees(ctx context.Context, qtx *db.Queries, outletID pgtype.UUID, roles []string, title, body, notifType string, relatedEntityID pgtype.UUID) error {
	employees, err := qtx.ListEmployeesByOutletAndRole(ctx, db.ListEmployeesByOutletAndRoleParams{
		OutletID: outletID,
		Roles:    roles,
	})
	if err != nil {
		return err
	}

	for _, emp := range employees {
		if err := NotifyEmployee(ctx, qtx, emp.ID, title, body, notifType, relatedEntityID); err != nil {
			return err
		}
	}
	return nil
}
