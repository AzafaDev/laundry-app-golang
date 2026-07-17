package employee

import (
	"errors"
	"laundry-app-with-golang/internal/apperr"
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (h *Handler) CreateEmployee(c *gin.Context) {
	var req CreateEmployeeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	if len(req.FullName) == 0 {
		apperr.RespondError(c, http.StatusBadRequest, "full_name_required")
		return
	}

	password := strings.TrimSpace(req.Password)
	inviteMode := password == ""

	var passwordHash pgtype.Text
	if !inviteMode {
		if len(password) < 8 {
			apperr.RespondError(c, http.StatusBadRequest, "password_too_short")
			return
		}

		hashed, err := auth.HashPassword(password)
		if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		passwordHash = pgtype.Text{String: hashed, Valid: true}
	}

	var outletID pgtype.UUID
	if req.Role == "outlet_admin" {
		if req.OutletID == nil || *req.OutletID == "" {
			apperr.RespondError(c, http.StatusBadRequest, "outlet_id_required_for_outlet_admin")
			return
		}
	}
	if req.OutletID != nil && *req.OutletID != "" {
		if err := outletID.Scan(*req.OutletID); err != nil {
			apperr.RespondError(c, http.StatusBadRequest, "invalid_outlet_id")
			return
		}
		if _, err := h.Queries.GetOutletByID(c.Request.Context(), outletID); errors.Is(err, pgx.ErrNoRows) {
			apperr.RespondError(c, http.StatusBadRequest, "outlet_not_found")
			return
		} else if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
	}

	created, err := h.Queries.CreateEmployee(c.Request.Context(), db.CreateEmployeeParams{
		FullName:     req.FullName,
		Email:        req.Email,
		Phone:        pgtype.Text{String: req.Phone, Valid: req.Phone != ""},
		PasswordHash: passwordHash,
		Role:         req.Role,
		IsActive:     !inviteMode,
		OutletID:     outletID,
	})

	if isUniqueViolation(err) {
		apperr.RespondError(c, http.StatusConflict, "email_already_registered")
		return
	}

	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	inviteSent := false
	if inviteMode {
		token, err := auth.GenerateRandomToken()
		if err != nil {
			log.Printf("error generating invite token: %v", err)
		} else {
			_, err = h.Queries.CreateEmployeePasswordResetToken(c.Request.Context(), db.CreateEmployeePasswordResetTokenParams{
				EmployeeID: created.ID,
				TokenHash:  auth.HashToken(token),
				ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
			})
			if err != nil {
				log.Printf("error creating invite token: %v", err)
			} else if err := h.emailClient.SendEmployeePasswordResetEmail(created.Email, token); err != nil {
				log.Printf("error sending invite email: %v", err)
			} else {
				inviteSent = true
			}
		}
	}

	resp := toEmployeeResponse(created)
	resp.InviteSent = inviteSent
	resp.Message = "employee created successfully!"

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ResendInvite(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	existing, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "employee_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if existing.IsActive {
		apperr.RespondError(c, http.StatusBadRequest, "employee_already_active")
		return
	}

	if err := h.Queries.DeleteUnusedEmployeePasswordResetTokens(c.Request.Context(), employeeID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	inviteSent := false
	token, err := auth.GenerateRandomToken()
	if err != nil {
		log.Printf("error generating invite token: %v", err)
	} else {
		_, err = h.Queries.CreateEmployeePasswordResetToken(c.Request.Context(), db.CreateEmployeePasswordResetTokenParams{
			EmployeeID: existing.ID,
			TokenHash:  auth.HashToken(token),
			ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(1 * time.Hour), Valid: true},
		})
		if err != nil {
			log.Printf("error creating invite token: %v", err)
		} else if err := h.emailClient.SendEmployeePasswordResetEmail(existing.Email, token); err != nil {
			log.Printf("error sending invite email: %v", err)
		} else {
			inviteSent = true
		}
	}

	resp := toEmployeeResponse(existing)
	resp.InviteSent = inviteSent
	resp.Message = "undangan berhasil dikirim ulang!"

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) AssignEmployeeOutlet(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	var req AssignOutletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	existing, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "employee_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	var outletID pgtype.UUID
	if req.OutletID == nil {
		if existing.Role == "outlet_admin" {
			apperr.RespondError(c, http.StatusBadRequest, "outlet_admin_requires_outlet")
			return
		}
	} else {
		if err := outletID.Scan(*req.OutletID); err != nil {
			apperr.RespondError(c, http.StatusBadRequest, "invalid_outlet_id")
			return
		}
		if _, err := h.Queries.GetOutletByID(c.Request.Context(), outletID); errors.Is(err, pgx.ErrNoRows) {
			apperr.RespondError(c, http.StatusBadRequest, "outlet_not_found")
			return
		} else if err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
	}

	updated, err := h.Queries.UpdateEmployeeOutlet(c.Request.Context(), db.UpdateEmployeeOutletParams{
		OutletID: outletID,
		ID:       employeeID,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	resp := toEmployeeResponse(updated)
	resp.Message = "employee outlet updated successfully"
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListEmployees(c *gin.Context) {
	limit, offset := parsePagination(c)
	includeDeleted := c.Query("include_deleted") == "true"

	var role pgtype.Text
	if v := c.Query("role"); v != "" {
		role = pgtype.Text{String: v, Valid: true}
	}

	var search pgtype.Text
	if v := c.Query("search"); v != "" {
		search = pgtype.Text{String: v, Valid: true}
	}

	employees, err := h.Queries.ListEmployees(c.Request.Context(), db.ListEmployeesParams{
		IncludeDeleted: includeDeleted,
		Role:           role,
		Search:         search,
		RowLimit:       limit,
		RowOffset:      offset,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	totalCount, err := h.Queries.CountEmployees(c.Request.Context(), db.CountEmployeesParams{
		IncludeDeleted: includeDeleted,
		Role:           role,
		Search:         search,
	})
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	data := make([]EmployeeResponse, 0, len(employees))
	for _, e := range employees {
		data = append(data, toEmployeeResponseWithOutlet(e))
	}

	c.JSON(http.StatusOK, EmployeeListResponse{Data: data, TotalCount: totalCount})
}

func (h *Handler) GetEmployeeByIDAdmin(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	existing, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "employee_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, toEmployeeResponse(existing))
}

func (h *Handler) UpdateEmployee(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	var req UpdateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	if len(req.FullName) == 0 {
		apperr.RespondError(c, http.StatusBadRequest, "full_name_required")
		return
	}

	existing, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "employee_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if req.Role == "outlet_admin" && !existing.OutletID.Valid {
		apperr.RespondError(c, http.StatusBadRequest, "outlet_admin_requires_outlet_assign")
		return
	}

	updated, err := h.Queries.UpdateEmployee(c.Request.Context(), db.UpdateEmployeeParams{
		FullName: req.FullName,
		Phone:    pgtype.Text{String: req.Phone, Valid: req.Phone != ""},
		Role:     req.Role,
		ID:       employeeID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "employee_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if existing.Role != updated.Role {
		if err := h.Queries.RevokeEmployeeRefreshTokensByEmployeeID(c.Request.Context(), employeeID); err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
		if _, err := h.Queries.IncrementEmployeeTokenVersion(c.Request.Context(), employeeID); err != nil {
			apperr.RespondInternalError(c, err)
			return
		}
	}

	resp := toEmployeeResponse(updated)
	resp.Message = "employee updated successfully"
	c.JSON(http.StatusOK, resp)
}

// selfDeleteGuard reports whether targetID (parsed from a URL param) refers
// to the same employee as the one making the request. Both sides are parsed
// into pgtype.UUID and compared by .Bytes rather than as raw strings — the
// context value comes from claims.EmployeeID's canonical lowercase
// .String() form, while the URL param is caller-supplied and may be a
// differently-cased (but equal) UUID, which would false-negative under a
// plain string comparison and silently let an employee delete themselves.
func selfDeleteGuard(c *gin.Context, targetID pgtype.UUID) bool {
	callerIDStr, ok := c.MustGet("employee_id").(string)
	if !ok {
		return false
	}

	var callerID pgtype.UUID
	if err := callerID.Scan(callerIDStr); err != nil {
		return false
	}

	return callerID.Bytes == targetID.Bytes
}

func (h *Handler) SoftDeleteEmployee(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	if selfDeleteGuard(c, employeeID) {
		apperr.RespondError(c, http.StatusBadRequest, "cannot_delete_self")
		return
	}

	if _, err := h.Queries.GetEmployeeByID(c.Request.Context(), employeeID); errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "employee_not_found")
		return
	} else if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	tx, err := h.Pool.Begin(c.Request.Context())
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}
	defer tx.Rollback(c.Request.Context())

	qtx := h.Queries.WithTx(tx)

	if err := qtx.SoftDeleteEmployee(c.Request.Context(), employeeID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := qtx.RevokeEmployeeRefreshTokensByEmployeeID(c.Request.Context(), employeeID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if err := tx.Commit(c.Request.Context()); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "employee deleted successfully"})
}

func (h *Handler) HardDeleteEmployee(c *gin.Context) {
	var employeeID pgtype.UUID
	if err := employeeID.Scan(c.Param("id")); err != nil {
		apperr.RespondError(c, http.StatusBadRequest, "invalid_employee_id")
		return
	}

	if selfDeleteGuard(c, employeeID) {
		apperr.RespondError(c, http.StatusBadRequest, "cannot_delete_self")
		return
	}

	existing, err := h.Queries.GetEmployeeByIDAny(c.Request.Context(), employeeID)
	if errors.Is(err, pgx.ErrNoRows) {
		apperr.RespondError(c, http.StatusNotFound, "employee_not_found")
		return
	}
	if err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	if !existing.DeletedAt.Valid {
		apperr.RespondError(c, http.StatusBadRequest, "employee_must_be_deleted_first")
		return
	}

	if err := h.Queries.HardDeleteEmployee(c.Request.Context(), employeeID); err != nil {
		apperr.RespondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "employee permanently deleted"})
}
