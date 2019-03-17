package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"sync/atomic"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

type generic map[string]interface{}

var authorizationLock int32
var alphanumeric = regexp.MustCompile("^[a-zA-Z0-9_]+$")

func startWebserver() {
	e := echo.New()
	e.HidePort = true
	e.HideBanner = true
	e.Use(middleware.CORS())
	e.GET("/api/status", getStatus)
	e.GET("/api/register", getDeviceToken)
	e.GET("/api/getDevices", getDevices)
	e.POST("/api/queryData", queryData)
	e.POST("/api/queryDataRelative", queryDataRelative)
	e.POST("/api/updateDeviceName", postUpdateDeviceName)
	e.Static("/", "static")
	log.Println("[webapi] started http server on " + webserverEndpoint)
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
	return c.JSON(http.StatusOK, generic{"devices": deviceStorage.Devices})
}

func queryData(c echo.Context) error {
	authorized := checkAuthorization(c)
	if !authorized {
		return c.JSON(http.StatusOK, generic{"err": "Unauthorized"})
	}
	request := DataQueryRequest{}
	err := json.NewDecoder(c.Request().Body).Decode(&request)
	if err != nil {
		return c.JSON(http.StatusOK, generic{"err": "Could not decode request"})
	}
	if !alphanumeric.MatchString(request.DeviceID) {
		return c.JSON(http.StatusOK, generic{"err": "Bad device id field in request"})
	}
	from, to := time.Unix(int64(request.BeginUnix), 0), time.Unix(int64(request.EndUnix), 0)
	res := queryMetrics(request.DeviceID, request.SensorID, from, to, request.ResolutionSeconds)
	return c.JSON(http.StatusOK, generic{"datapoints": res})
}

func queryDataRelative(c echo.Context) error {
	authorized := checkAuthorization(c)
	if !authorized {
		return c.JSON(http.StatusOK, generic{"err": "Unauthorized"})
	}
	request := RelativeDataQueryRequest{}
	err := json.NewDecoder(c.Request().Body).Decode(&request)
	if err != nil {
		return c.JSON(http.StatusOK, generic{"err": "Could not decode request"})
	}
	if !alphanumeric.MatchString(request.DeviceID) {
		return c.JSON(http.StatusOK, generic{"err": "Bad device id field in request"})
	}
	now := time.Now()
	from := now.Add(time.Second * time.Duration(request.BeginRelativeSeconds))
	to := now.Add(time.Second * time.Duration(request.EndRelativeSeconds))
	res := queryMetrics(request.DeviceID, request.SensorID, from, to, request.ResolutionSeconds)
	return c.JSON(http.StatusOK, generic{"datapoints": res, "relativeTime": now})
}

func postUpdateDeviceName(c echo.Context) error {
	return c.JSON(http.StatusOK, generic{"err": nil})
}
