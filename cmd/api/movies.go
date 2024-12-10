package main

import (
	"errors"
	"fmt"
	"github.com/LTXWorld/greenLight_copy/internal/data"
	"github.com/LTXWorld/greenLight_copy/internal/validator"
	"net/http"
)

// 将传来的JSON请求转换为Go数据,并对JSON请求的格式以及其中具体数据进行校验是否出错
func (app *application) createMovieHandler(w http.ResponseWriter, r *http.Request) {
	// 声明一个匿名的结构体来保存请求体中的数据
	var input struct {
		Title   string       `json:"title"`
		Year    int32        `json:"year"`
		Runtime data.Runtime `json:"runtime"`
		Genres  []string     `json:"genres"`
	}

	// 反序列化到一个中间结构体input，后续有复制操作。
	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Copy the values from the input struct to a new Movie struct
	movie := &data.Movie{
		Title:   input.Title,
		Year:    input.Year,
		Runtime: input.Runtime,
		Genres:  input.Genres,
	}
	// 初始化一个新的Validator实例
	v := validator.New()

	// 对输入进行检查（上面readJSON不是已经检查了一遍了吗？)
	// readJSON中只是对JSON格式进行了检查，而这里是对每一个具体的属性进行检查,并给出对应的错误提示。
	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Call the Insert() passing in a pointer to the validated movie struct
	err = app.models.Movies.Insert(movie)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	// 发送HTTP响应，希望包含一个Location头部，让客户端知道可以在哪个URL找到新建资源
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/movies/%d", movie.ID))

	// Write a JSON response with a 201 Created status code
	err = app.writeJSON(w, http.StatusCreated, envelop{"movie": movie}, headers)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// 通过Get方法获取想要的record并封装在一个JSON中传给用户
func (app *application) showMovieHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Call the Get method to fetch the data for a specific movie
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r) // 404 NotFound
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Encode，将数据先封装在一个map中，再写进JSON去传输
	err = app.writeJSON(w, http.StatusOK, envelop{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// 更新流程是：先根据id读取传来的JSON中的数据去数据库中查是否存在，如果存在将JSON复制在input中，在将值从input拿到movie对象中，
// 检查是否符合要求，如果符合要求再将movie对象中的数据插入到数据库中，最后将movie对象中的数据写成JSON格式返回给用户
func (app *application) updateMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the movie ID from the URL
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Fetch the existing movie record from the database
	movie, err := app.models.Movies.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r) // 404 NotFound
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Declare an input struct to hold the expected data from the client
	// Use the pointers in order to change partial record
	var input struct {
		Title   *string       `json:"title"`
		Year    *int32        `json:"year"`
		Runtime *data.Runtime `json:"runtime"`
		Genres  []string      `json:"genres"`
	}

	// Read the JSON request body data into the input struct
	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Copy the values from request body to the movie record
	// If the input.Title value is nil that means no corresponding "title" kv pair war provided in JSON body
	// So we move on and leave the movie record unchanged, only change those filed which are not nil
	if input.Title != nil {
		movie.Title = *input.Title
	}
	if input.Year != nil {
		movie.Year = *input.Year
	}
	if input.Runtime != nil {
		movie.Runtime = *input.Runtime
	}
	if input.Genres != nil {
		movie.Genres = input.Genres
	}

	// Validate the updated movie record
	v := validator.New()

	if data.ValidateMovie(v, movie); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Pass the updated record to Databases
	// Update use the version to prevent data race
	err = app.models.Movies.Update(movie)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrEditConflict):
			app.editConflictResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Write the uploaded movie record as a JSON response
	err = app.writeJSON(w, http.StatusOK, envelop{"movie": movie}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// 删除指定id的movie，并返回删除成功信息
func (app *application) deleteMovieHandler(w http.ResponseWriter, r *http.Request) {
	// Extract the movie
	id, err := app.readIDParam(r)
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	// Delete the movie from the database
	err = app.models.Movies.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r) // 404 NotFound
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// Return a 200 ok status code
	err = app.writeJSON(w, http.StatusOK, envelop{"message": "movie successfully deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// 列出请求体中指定类型，名称，页码等的各个符合条件的movies信息，存储在HTTP响应中
func (app *application) listMoviesHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title        string
		Genres       []string
		data.Filters // 嵌入结构体页面等信息需要复用
	}

	v := validator.New()

	qs := r.URL.Query()

	// 会将black+panther转换为black panther
	input.Title = app.readString(qs, "title", "") // 在 URL 查询参数中，+ 号通常会被解释为空格
	input.Genres = app.readCSV(qs, "genres", []string{})

	//
	input.Filters.Page = app.readInt(qs, "page", 1, v)
	input.Filters.PageSize = app.readInt(qs, "page_size", 20, v)

	input.Filters.Sort = app.readString(qs, "sort", "id")
	// Add the supported sort values for this endpoint to the sort safelist
	input.Filters.SortSafelist = []string{"id", "title", "year", "runtime", "-id", "-title", "-year", "-runtime"}

	// ValidateFilters中有一堆check,Valid会检查这些check的结果是否最终有错误发生
	if data.ValidateFilters(v, input.Filters); !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Call the GetAll() method to retrieve the movies, passing in the various filter parameters.
	movies, metadata, err := app.models.Movies.GetAll(input.Title, input.Genres, input.Filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelop{"movies": movies, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
