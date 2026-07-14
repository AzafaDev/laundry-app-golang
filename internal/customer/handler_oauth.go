package customer

import (
	"laundry-app-with-golang/internal/auth"
	db "laundry-app-with-golang/internal/db/generated"
	oauthpkg "laundry-app-with-golang/internal/oauth"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) GoogleLogin(c *gin.Context) {
	state, err := auth.GenerateRandomToken()
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, h.Config.FrontendURL+"/auth/callback?success=false")
		return
	}

	c.SetSameSite(h.cookieSameSite())
	c.SetCookie("oauth_state", state, 300, "/", "", h.cookieSecure(), true)

	c.Redirect(http.StatusTemporaryRedirect, h.googleClient.AuthCodeURL(state))
}

func (h *Handler) GoogleCallback(c *gin.Context) {
	failRedirect := h.Config.FrontendURL + "/auth/callback?success=false"

	cookieState, err := c.Cookie("oauth_state")
	if err != nil || cookieState == "" {
		c.Redirect(http.StatusTemporaryRedirect, failRedirect)
		return
	}

	queryState := c.Query("state")
	if queryState == "" || queryState != cookieState {
		c.Redirect(http.StatusTemporaryRedirect, failRedirect)
		return
	}

	c.SetSameSite(h.cookieSameSite())
	c.SetCookie("oauth_state", "", -1, "/", "", h.cookieSecure(), true)

	code := c.Query("code")
	if code == "" {
		c.Redirect(http.StatusTemporaryRedirect, failRedirect)
		return
	}

	token, err := h.googleClient.Exchange(c.Request.Context(), code)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, failRedirect)
		return
	}

	userInfo, err := h.googleClient.FetchUserInfo(c.Request.Context(), token)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, failRedirect)
		return
	}

	if !userInfo.VerifiedEmail {
		c.Redirect(http.StatusTemporaryRedirect, failRedirect)
		return
	}

	customer, err := h.resolveOAuthCustomer(c, userInfo)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, failRedirect)
		return
	}

	if _, _, err := h.issueTokens(c, customer.ID, customer.TokenVersion); err != nil {
		c.Redirect(http.StatusTemporaryRedirect, failRedirect)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, h.Config.FrontendURL+"/auth/callback?success=true")
}

func (h *Handler) resolveOAuthCustomer(c *gin.Context, userInfo *oauthpkg.UserInfo) (db.Customer, error) {
	ctx := c.Request.Context()

	socialAccount, err := h.Queries.GetSocialAccountByProviderAndUID(ctx, db.GetSocialAccountByProviderAndUIDParams{
		Provider:    "google",
		ProviderUid: userInfo.ID,
	})
	if err == nil {
		return h.Queries.GetCustomerByID(ctx, socialAccount.CustomerID)
	}

	existingCustomer, err := h.Queries.GetCustomerByEmail(ctx, userInfo.Email)
	if err == nil {
		if _, err := h.Queries.CreateSocialAccount(ctx, db.CreateSocialAccountParams{
			CustomerID:  existingCustomer.ID,
			Provider:    "google",
			ProviderUid: userInfo.ID,
		}); err != nil {
			return db.Customer{}, err
		}
		return existingCustomer, nil
	}

	newCustomer, err := h.Queries.CreateOAuthCustomer(ctx, db.CreateOAuthCustomerParams{
		FullName: userInfo.Name,
		Email:    userInfo.Email,
	})

	if isUniqueViolation(err) {
		newCustomer, err = h.Queries.GetCustomerByEmail(ctx, userInfo.Email)
		if err != nil {
			return db.Customer{}, err
		}
	} else if err != nil {
		return db.Customer{}, err
	}

	if _, err := h.Queries.CreateSocialAccount(ctx, db.CreateSocialAccountParams{
		CustomerID:  newCustomer.ID,
		Provider:    "google",
		ProviderUid: userInfo.ID,
	}); err != nil {
		return db.Customer{}, err
	}

	return newCustomer, nil
}
