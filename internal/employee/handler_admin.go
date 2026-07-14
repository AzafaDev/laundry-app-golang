package employee

import (
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "full name is required"})
		return
	}

	password := strings.TrimSpace(req.Password)
	inviteMode := password == ""

	var passwordHash pgtype.Text
	if !inviteMode {
		if len(password) < 8 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
			return
		}

		hashed, err := auth.HashPassword(password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		passwordHash = pgtype.Text{String: hashed, Valid: true}
	}

	created, err := h.Queries.CreateEmployee(c.Request.Context(), db.CreateEmployeeParams{
		FullName:     req.FullName,
		Email:        req.Email,
		Phone:        pgtype.Text{String: req.Phone, Valid: req.Phone != ""},
		PasswordHash: passwordHash,
		Role:         req.Role,
		IsActive:     !inviteMode,
	})

	if isUniqueViolation(err) {
		c.JSON(http.StatusConflict, gin.H{"error": "email has been registered"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	resp := EmployeeResponse{
		ID:         created.ID.String(),
		FullName:   created.FullName,
		Email:      created.Email,
		Role:       created.Role,
		InviteSent: inviteSent,
		Message:    "employee created successfully!",
	}

	c.JSON(http.StatusCreated, resp)
}
