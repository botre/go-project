package main

import (
	"encoding/json"
	"github.com/antoniodipinto/ikisocket"
	"github.com/atrox/haikunatorgo/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/template/html"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"go-project/src/database"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

const port = 8080

var isProduction = os.Getenv("APPLICATION_ENV") == "production"

func main() {
	/* Database */
	db := database.Connect()

	/* Haiku maker */
	haikuMaker := haikunator.New()

	/* Server */

	engine := html.New("./src/views", ".html")

	if !isProduction {
		engine.Reload(true)
		engine.Debug(true)
	}

	application := fiber.New(fiber.Config{
		Views:       engine,
		ViewsLayout: "layouts/main",
	})

	application.Use(limiter.New(
		limiter.Config{
			Max:        125,
			Expiration: 1 * time.Minute,
		}))

	application.Use(compress.New())

	// Static handling

	application.Static("/", "./public")

	// WS handling

	application.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	application.Get("/ws/:endpoint", ikisocket.New(func(kws *ikisocket.Websocket) {
		endpointID := kws.Params("endpoint")
		// TODO: create or update
		database.CreateSocketClient(db, &database.SocketClient{
			UUID:       kws.UUID,
			EndpointID: endpointID,
		})
		log.Printf("%s connected to WS\n", endpointID)
	}))

	ikisocket.On(ikisocket.EventDisconnect, func(ep *ikisocket.EventPayload) {
		database.DeleteSocketClientForUUID(db, ep.Kws.UUID)
	})

	ikisocket.On(ikisocket.EventClose, func(ep *ikisocket.EventPayload) {
		database.DeleteSocketClientForUUID(db, ep.Kws.UUID)
	})

	// HTTP handling

	application.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			"Title": "Home",
		})
	})

	application.Get("/favicon", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNotFound)
	})

	application.Get("/robots", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNotFound)
	})

	// TODO: hide sensitive data in production
	application.Get("/api/debug", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"host":         string(c.Request().Host()),
			"isProduction": isProduction,
			"requests":     len(database.GetRequests(db)),
			"sockets":      len(database.GetSocketClients(db)),
		})
	})

	application.Get("/api/endpoints/:endpoint/requests", func(c *fiber.Ctx) error {
		endpointID := c.Params("endpoint")
		return c.JSON(fiber.Map{
			"requests": database.GetRequestsForEndpointID(db, endpointID),
		})
	})

	application.Get("/:endpoint", func(c *fiber.Ctx) error {
		endpointID := c.Params("endpoint")
		host := string(c.Request().Host())
		protocol := c.Protocol()
		websocketProtocol := "ws"
		if protocol == "https" {
			websocketProtocol = "wss"
		}
		return c.Render("endpoint", fiber.Map{
			"Title":                "Endpoint",
			"EndpointID":           endpointID,
			"EndpointURL":          protocol + "://" + host + "/to/" + endpointID,
			"EndpointWebSocketURL": websocketProtocol + "://" + host + "/ws/" + endpointID,
		})
	})

	application.Post("/endpoint", func(c *fiber.Ctx) error {
		endpointID := haikuMaker.Haikunate()
		log.Printf("Created endpoint %s\n", endpointID)
		return c.Redirect("/" + endpointID)
	})

	application.Use("/to/:endpoint", func(c *fiber.Ctx) error {
		UUID := uuid.NewString()
		endpointID := c.Params("endpoint")
		method := c.Method()
		path := c.Path()
		ip := c.IP()
		body := c.Body()
		// TODO: handle error
		headers, _ := json.Marshal(c.GetReqHeaders())
		request := database.Request{
			UUID:       UUID,
			EndpointID: endpointID,
			Method:     method,
			Path:       path,
			IP:         ip,
			Body:       string(body),
			Headers:    string(headers),
		}
		database.CreateRequest(db, &request)
		socketClients := database.GetSocketClientsForEndpointID(db, endpointID)
		for _, socketClient := range socketClients {
			// TODO: error handling
			marshalled, _ := json.Marshal(request)
			ikisocket.EmitTo(socketClient.UUID, marshalled)
		}
		return c.SendStatus(http.StatusOK)
	})

	application.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusNotFound)
	})

	host := "localhost:"
	if isProduction {
		host = ":"
	}
	log.Fatalln(application.Listen(host + strconv.Itoa(port)))
}
