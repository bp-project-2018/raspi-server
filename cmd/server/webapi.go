package main

import (
	"context"
	"net/http"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

type generic map[string]interface{}

func startWebserver() {
	e := echo.New()
	e.HidePort = true
	e.HideBanner = true
	e.Use(middleware.CORS())
	e.GET("/api/status", getStatus)
	e.GET("/api/register", getDeviceToken)
	e.GET("/api/getDevices", getDevices)
	e.POST("/api/getData", getData)
	e.POST("/api/updateDeviceName", postUpdateDeviceName)
	e.Static("/", "static")
	e.Logger.Fatal(e.Start(webserverEndpoint))
}

func checkAuthorization(c echo.Context) bool {
	token := c.Request().Header.Get("Authorization")
	return verifyToken(token)
}

func getStatus(c echo.Context) error {
	authorized := checkAuthorization(c)
	if !authorized {
		return c.JSON(http.StatusOK, generic{"err": "Unauthorized"})
	}
	return c.JSON(http.StatusOK, generic{"err": nil})
}

func getDeviceToken(c echo.Context) error {
	timeout, cancel := context.WithTimeout(context.Background(), buttonTimeout)
	defer cancel()
	authorized := HardwareWaitForPairingButton(timeout)
	if !authorized {
		return c.JSON(http.StatusOK, generic{"err": "Timeout"})
	}
	return c.JSON(http.StatusOK, generic{"token": generateToken()})
}

func getDevices(c echo.Context) error {
	authorized := checkAuthorization(c)
	if !authorized {
		return c.JSON(http.StatusOK, generic{"err": "Unauthorized"})
	}
	return c.JSON(http.StatusOK, generic{"devices": []generic{}})
}

func getData(c echo.Context) error {
	return c.JSON(http.StatusOK, generic{"err": nil})
}

func postUpdateDeviceName(c echo.Context) error {
	return c.JSON(http.StatusOK, generic{"err": nil})
}
