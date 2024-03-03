package main

import (
	"Sparkle/cleanup"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

func REST() {
	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	}), middleware.GzipWithConfig(middleware.DefaultGzipConfig), middleware.Logger(), middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Hello, World!")
	})
	e.Static("/static", OUTPUT)
	e.GET("/all", func(c echo.Context) error {
		ctx := c.Request().Context()
		keys, err := rdb.Do(ctx, rdb.B().Keys().Pattern("job:*").Build()).ToArray()
		if err != nil {
			log.Errorf("error getting keys: %v", err)
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, keys)
	})
	e.GET("/job", func(c echo.Context) error {
		name := c.QueryParam("name")
		ctx := c.Request().Context()
		err := rdb.Do(ctx, rdb.B().JsonGet().Key(name).Build()).Error()
		if err != nil {
			log.Errorf("error getting job: %v", err)
			if err.Error() == "redis nil message" {
				return c.String(http.StatusNotFound, "Job not found")
			}
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.String(http.StatusOK, "Job: "+name)
	})
	cleanup.AddOnStopFunc(cleanup.Echo, func(_ os.Signal) {
		err := e.Close()
		if err != nil {
			return
		}
	})
	e.Logger.Fatal(e.Start(":1323"))
}
