package art

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stripe/stripe-go/v72"
	//portalsession "github.com/stripe/stripe-go/v72/billingportal/session"
	"github.com/stripe/stripe-go/v72/checkout/session"
	"github.com/stripe/stripe-go/v72/webhook"
	"html/template"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func index(w http.ResponseWriter, r *http.Request) {
	indexFile := filepath.Join("assets", "build", "index.html")

	tmpl, err := template.New("").ParseFiles(indexFile)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
		return
	}
	// check your err
	err = tmpl.ExecuteTemplate(w, "index", nil)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func settings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s := Settings{
		PriceId:   priceID,
		StripeKey: stripePublicKey,
	}

	err := json.NewEncoder(w).Encode(s)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func login(w http.ResponseWriter, r *http.Request) {

}

func logout(w http.ResponseWriter, r *http.Request) {

}

func register(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Price    string `json:"priceId"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Printf("json.NewDecoder.Decode: %v", err)
		return
	}

	// See https://stripe.com/docs/api/checkout/sessions/create
	// for additional parameters to pass.
	// {CHECKOUT_SESSION_ID} is a string literal; do not change it!
	// the actual Session ID is returned in the query parameter when your customer
	// is redirected to the success page.
	params := &stripe.CheckoutSessionParams{
		SuccessURL: &successUrl,
		CancelURL:  &cancelUrl,
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			&stripe.CheckoutSessionLineItemParams{
				Price: stripe.String(req.Price),
				// For metered billing, do not pass quantity
				Quantity: stripe.Int64(1),
			},
		},
	}

	s, err := session.New(params)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, struct {
			ErrorData string `json:"error"`
		}{
			ErrorData: "test",
		})
		return
	}

	writeJSON(w, struct {
		SessionID string `json:"sessionId"`
	}{
		SessionID: s.ID,
	})
}

func forgotPassword(w http.ResponseWriter, r *http.Request) {

}

func resetPassword(w http.ResponseWriter, r *http.Request) {

}

func authSettings(w http.ResponseWriter, r *http.Request) {

}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("ioutil.ReadAll: %v", err)
		return
	}

	event, err := webhook.ConstructEvent(b, r.Header.Get("Stripe-Signature"), webhookSecret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf("webhook.ConstructEvent: %v", err)
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		log.Println("Payment is successful and the subscription is created.")
		// Payment is successful and the subscription is created.
		// You should provision the subscription and save the customer ID to your database.
	case "invoice.paid":
		log.Println("Payment for a subscription is made.")
		// Continue to provision the subscription as payments continue to be made.
		// Store the status in your database and check when a user accesses your service.
		// This approach helps you avoid hitting rate limits.
	case "invoice.payment_failed":
		log.Println("The payment failed or the customer does not have a valid payment method. The subscription is now past due.")
		// The payment failed or the customer does not have a valid payment method.
		// The subscription becomes past_due. Notify your customer and send them to the
		// customer portal to update their payment information.
	default:
		// unhandled event type
		log.Println("Unhandled event type:")
		log.Println(event.Type)
	}
}

func generate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	err := r.ParseMultipartForm(10 * 1024 * 1024)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	var img image.Image

	imageType := r.FormValue("type")
	if imageType == "upload" {

		uploaded, uploadHeader, err := r.FormFile("fileUpload")
		if err != nil {
			log.Println(err)
			http.Error(w, http.StatusText(500), 500)
			return
		}
		defer uploaded.Close()
		buffer := make([]byte, 512)
		_, err = uploaded.Read(buffer)
		if err != nil {
			fmt.Println(err)
		}
		_, err = uploaded.Seek(0, 0)
		if err != nil {
			fmt.Println(err)
		}

		contentType := http.DetectContentType(buffer)

		if contentType != "image/jpeg" && contentType != "image/png" {
			log.Println(contentType)
			http.Error(w, http.StatusText(422), 422)
			return
		}
		if contentType == "image/jpeg" {
			img, err = jpeg.Decode(uploaded)
			if err != nil {
				log.Println(err)
				http.Error(w, http.StatusText(500), 500)
				return
			}
		}

		if contentType == "image/png" {
			img, err = png.Decode(uploaded)
			if err != nil {
				x, err2 := ioutil.ReadAll(uploaded)
				if err2 != nil {
					log.Println(err)
					http.Error(w, http.StatusText(500), 500)
					return
				}
				s := string(x)
				log.Println(s[0:50] + "..." + s[len(string(x))-50:])
				log.Println(uploadHeader.Header)
				log.Println(err)
				http.Error(w, http.StatusText(500), 500)
				return
			}
		}

	}

	width := r.FormValue("width")
	height := r.FormValue("height")
	shapes := r.FormValue("shapes")
	shapeStroke := r.FormValue("shapeStroke")
	triangulate := r.FormValue("triangulate")
	triangulateBefore := r.FormValue("triangulateBefore")
	strokeThickness := r.FormValue("strokeThickness")
	complexityAmount := r.FormValue("complexityAmount")
	min := r.FormValue("min")
	max := r.FormValue("max")
	maxPoints := r.FormValue("maxPoints")
	pointsThreshold := r.FormValue("pointsThreshold")
	sobelThreshold := r.FormValue("sobelThreshold")
	triangulateWireframe := r.FormValue("triangulateWireframe")
	triangulateNoise := r.FormValue("triangulateNoise")
	triangulateGrayscale := r.FormValue("triangulateGrayscale")

	wi, err := strconv.Atoi(width)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	hi, err := strconv.Atoi(height)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	shapesBool, err := strconv.ParseBool(shapes)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	shapesStrokeBool, err := strconv.ParseBool(shapeStroke)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	triangulateBool, err := strconv.ParseBool(triangulate)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	triangulateBeforeBool, err := strconv.ParseBool(triangulateBefore)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	triangulateNoiseBool, err := strconv.ParseBool(triangulateNoise)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	triangulateWireframeBool, err := strconv.ParseBool(triangulateWireframe)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	triangulateGrayscaleBool, err := strconv.ParseBool(triangulateGrayscale)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	strokeThicknessInt, err := strconv.Atoi(strokeThickness)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	complexityAmountInt, err := strconv.Atoi(complexityAmount)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	minInt, err := strconv.Atoi(min)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	maxInt, err := strconv.Atoi(max)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	maxPointsInt, err := strconv.Atoi(maxPoints)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	pointsThresholdInt, err := strconv.Atoi(pointsThreshold)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	sobelThresholdInt, err := strconv.Atoi(sobelThreshold)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}

	if wi > 2000 || hi > 2000 {
		http.Error(w, "max size is 1200x1200", 500)
		return
	}

	if minInt > 10 || minInt < 3 {
		http.Error(w, "min invalid", 500)
		return
	}

	if maxInt > 10 || maxInt < 3 {
		http.Error(w, "max invalid", 500)
		return
	}

	if complexityAmountInt > 100 || complexityAmountInt < 1 {
		http.Error(w, "complexity invalid", 500)
		return
	}

	if strokeThicknessInt > 10 || strokeThicknessInt < 1 {
		log.Println(strokeThickness)
		http.Error(w, "stroke invalid", 500)
		return
	}

	if maxPointsInt > 5000 || maxPointsInt < 500 {
		log.Println(maxPoints)
		http.Error(w, "max points invalid", 500)
		return
	}

	if pointsThresholdInt > 30 || pointsThresholdInt < 10 {
		log.Println(pointsThreshold)
		http.Error(w, "point threshold invalid", 500)
		return
	}

	if sobelThresholdInt > 20 || sobelThresholdInt < 5 {
		log.Println(sobelThresholdInt)
		http.Error(w, "sobel threshold invalid", 500)
		return
	}

	log.Printf("Generate called from %s\n", ip)

	mutex.Lock()
	id := generateUniqueId(queue, 10)
	queue = append(queue, id)
	resp := GeneratePollResponse{
		Queue:      len(queue),
		Link:       "",
		Identifier: id,
	}
	job := Image{
		Identifier: id,
		Timestamp:  time.Now(),
		// TODO if we run this behind a load balancer the IP will be local so we have to adapt
		// TODO hash the IP so we don't store PII
		RequestIP:            ip,
		Width:                wi,
		Height:               hi,
		ImageType:            imageType,
		Shapes:               shapesBool,
		Max:                  maxInt,
		Min:                  minInt,
		ComplexityAmount:     complexityAmountInt,
		StrokeThickness:      strokeThicknessInt,
		Triangulate:          triangulateBool,
		TriangulateBefore:    triangulateBeforeBool,
		ShapesStroke:         shapesStrokeBool,
		Image:                img,
		MaxPoints:            maxPointsInt,
		SobelThreshold:       sobelThresholdInt,
		PointsThreshold:      pointsThresholdInt,
		TriangulateWireframe: triangulateWireframeBool,
		TriangulateGrayscale: triangulateGrayscaleBool,
		TriangulateNoise:     triangulateNoiseBool,
	}
	jobChan <- job
	mutex.Unlock()
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
		return
	}
}

func img(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	res := ""
	mutex.Lock()
	if val, ok := images[vars["id"]]; ok {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil && ip == val.RequestIP {
			res = fmt.Sprintf("%s/%s", outDir, val.FileName)
		} else {
			log.Println(errors.New(fmt.Sprintf("%s did not match %s", r.RemoteAddr, val.RequestIP)))
			http.Error(w, http.StatusText(500), 500)
			return
		}
	}
	mutex.Unlock()
	img, err := os.Open(res)
	if err != nil {
		log.Println(err)
		http.Error(w, http.StatusText(500), 500)
		return
	}
	defer img.Close()
	w.Header().Set("Content-Type", "image/png")
	io.Copy(w, img)
}

func generatePoll(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.Header().Set("Content-Type", "application/json")
	mutex.Lock()
	res := ""
	id := vars["id"]
	i := indexOf(id, queue)
	resp := GeneratePollResponse{
		Queue: i + 1,
	}
	if i == -1 {
		if _, ok := images[id]; ok {
			res = fmt.Sprintf("/api/v1/img/%s.png", id)
			resp.Link = res
		}
		if currentJob.Identifier == id && currentJob.RandomImage {
			resp.Thumbnail = currentJob.Thumbnail
			resp.Description = currentJob.Description
			resp.RandomImage = currentJob.RandomImage
			resp.UserLocation = currentJob.UserLocation
			resp.UserName = currentJob.UserName
			resp.UserLink = currentJob.UserLink
			resp.ThumbnailLink = currentJob.ThumbnailLink
		}
	}
	mutex.Unlock()
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, http.StatusText(500), 500)
		return
	}
}
