package main

import (
	"context"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

type generic map[string]interface{}

var authorizationLock int32

func startWebserver() {
	e := echo.New()
	e.HidePort = true
	e.HideBanner = true
	e.Use(middleware.CORS())
	e.GET("/api/status", getStatus)
	e.GET("/api/register", getDeviceToken)
	e.GET("/api/getDevices", getDevices)
	e.GET("/api/getData", getData)
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

	if !atomic.CompareAndSwapInt32(&authorizationLock, 0, 1) {
		// another authorization process is currently running
		return c.JSON(http.StatusOK, generic{"err": "Another authorization process is already running"})
	}
	defer atomic.StoreInt32(&authorizationLock, 0)

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
	authorized := checkAuthorization(c)
	if !authorized {
		return c.JSON(http.StatusOK, generic{"err": "Unauthorized"})
	}
	deviceID, sensorID, precision := c.Param("deviceId"), c.Param("sensorId"), c.Param("precision")
	fromInt, err := strconv.Atoi(c.Param("from"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, generic{"err": "Bad 'from' field"})
	}
	toInt, err := strconv.Atoi(c.Param("to"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, generic{"err": "Bad 'to' field"})
	}
	from, to := time.Unix(int64(fromInt), 0), time.Unix(int64(toInt), 0)
	res := queryMetrics(deviceID, sensorID, from, to, precision)
	return c.JSON(http.StatusOK, res)
}

func postUpdateDeviceName(c echo.Context) error {
	return c.JSON(http.StatusOK, generic{"err": nil})
}
