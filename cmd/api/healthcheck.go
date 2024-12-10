package main

import (
	"net/http"
)

func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
	// 假设一个map作为我们要传输的类型
	data := envelop{
		"status": "available",
		"system_info": map[string]string{
			"environment": app.config.env,
			"version":     version,
		},
	}

	//// Add a 4 seconds delay to test shutdown
	//time.Sleep(4 * time.Second)

	err := app.writeJSON(w, http.StatusOK, data, nil)
	if err != nil {
		app.logger.PrintError(err, nil)
		app.serverErrorResponse(w, r, err)
	}
}
