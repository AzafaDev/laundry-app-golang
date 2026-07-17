package order_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"laundry-app-with-golang/internal/order"
	"laundry-app-with-golang/internal/testutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newComplaintMultipartRequest(t *testing.T, url, complaintType, description string, photos [][2]string) (*http.Request, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("complaint_type", complaintType); err != nil {
		t.Fatalf("failed to write complaint_type field: %v", err)
	}
	if err := writer.WriteField("description", description); err != nil {
		t.Fatalf("failed to write description field: %v", err)
	}

	for _, photo := range photos {
		contentType, content := photo[0], photo[1]
		part, err := writer.CreatePart(map[string][]string{
			"Content-Disposition": {`form-data; name="photos"; filename="photo.jpg"`},
			"Content-Type":        {contentType},
		})
		if err != nil {
			t.Fatalf("failed to create photo part: %v", err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write photo content: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, url, body)
	return req, writer.FormDataContentType()
}

// a real 1x1 pixel JPEG, since Cloudinary validates that uploaded bytes
// actually decode as an image before accepting them.
var tinyJPEG = func() []byte {
	data, err := base64.StdEncoding.DecodeString("/9j/4AAQSkZJRgABAQEAYABgAAD/2wBDAAMCAgICAgMCAgIDAwMDBAYEBAQEBAgGBgUGCQgKCgkICQkKDA8MCgsOCwkJDRENDg8QEBEQCgwSExIQEw8QEBD/2wBDAQMDAwQDBAgEBAgQCwkLEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBD/wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAj/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwCdABmX/9k=")
	if err != nil {
		panic(err)
	}
	return data
}()

func TestCreateComplaint_WithValidPhotos_UploadsAndReturnsCloudinaryURLs(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusReceivedByCustomer)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	url := fmt.Sprintf("/api/v1/customer/orders/%s/complaint", testOrder.ID.String())
	req, contentType := newComplaintMultipartRequest(t, url, "damaged", "baju robek", [][2]string{
		{"image/jpeg", string(tinyJPEG)},
		{"image/jpeg", string(tinyJPEG)},
	})
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-CSRF-Token", testutil.CookieValue(cookies, "csrf_token"))
	for _, ck := range cookies {
		req.AddCookie(ck)
	}

	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp order.ComplaintResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.PhotoURLs) != 2 {
		t.Fatalf("expected 2 photo URLs, got %d: %v", len(resp.PhotoURLs), resp.PhotoURLs)
	}
	for _, u := range resp.PhotoURLs {
		if !bytes.Contains([]byte(u), []byte("res.cloudinary.com")) {
			t.Errorf("expected a real cloudinary URL, got %q", u)
		}
	}
}

func TestCreateComplaint_OversizedPhoto_IsRejected(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusReceivedByCustomer)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	oversized := make([]byte, (2<<20)+1)

	url := fmt.Sprintf("/api/v1/customer/orders/%s/complaint", testOrder.ID.String())
	req, contentType := newComplaintMultipartRequest(t, url, "damaged", "baju robek", [][2]string{
		{"image/jpeg", string(oversized)},
	})
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-CSRF-Token", testutil.CookieValue(cookies, "csrf_token"))
	for _, ck := range cookies {
		req.AddCookie(ck)
	}

	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateComplaint_InvalidContentType_IsRejected(t *testing.T) {
	app := testutil.NewTestApp(t)

	customer := app.CreateTestCustomer(t)
	outlet := app.CreateTestOutlet(t)
	address := app.CreateTestAddress(t, customer.ID)
	testOrder := app.CreateTestOrder(t, customer.ID, outlet.ID, address.ID, order.StatusReceivedByCustomer)

	cookies := testutil.LoginAs(t, app.Router, "/api/v1/customer/auth/login", customer.Email, testutil.TestPassword)

	url := fmt.Sprintf("/api/v1/customer/orders/%s/complaint", testOrder.ID.String())
	req, contentType := newComplaintMultipartRequest(t, url, "damaged", "baju robek", [][2]string{
		{"application/pdf", "not an image"},
	})
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-CSRF-Token", testutil.CookieValue(cookies, "csrf_token"))
	for _, ck := range cookies {
		req.AddCookie(ck)
	}

	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
