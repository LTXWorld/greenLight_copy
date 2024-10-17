package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/LTXWorld/greenLight_copy/internal/validator"
	"github.com/lib/pq"
	"time"
)

type Movie struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"-"`
	Title     string    `json:"title"`
	Year      int32     `json:"year,omitempty"`
	Runtime   Runtime   `json:"runtime,omitempty"`
	Genres    []string  `json:"genres,omitempty"` // 电影的类型切片
	Version   int32     `json:"version"`
}

type MovieModel struct {
	DB *sql.DB
}

// Insert 这些CRUD方法的接收者没有使用指针类型是因为——一般只有需要更改接收者结构体中的字段时（或者结构体太大复制开销大）
// 本例中MovieModel结构体只有DB这个字段
// Add a placeholder method for insert
func (m MovieModel) Insert(movie *Movie) error {
	// 插入一条新记录的SQL语句，并返回信息（Postgresql专有)
	query := `
			INSERT INTO movies (title, year, runtime, genres)
			VALUES ($1, $2, $3, $4)
			RETURNING id, created_at, version`

	// 创建一个代表着占位符的movie中的属性切片
	args := []interface{}{movie.Title, movie.Year, movie.Runtime, pq.Array(movie.Genres)}

	// Create a context with a 3-second timeout
	ctx, cancle := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancle()

	// 使用QueryRow方法执行,并使用Scan方法将返回值注入到movie的三个属性中
	return m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.ID, &movie.CreatedAt, &movie.Version)
}

func (m MovieModel) Get(id int64) (*Movie, error) {
	// 健壮性判断
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	// Define the SQL query for retrieving the movie data.
	query := `
			SELECT id, created_at, title, year, runtime, genres, version
			FROM movies
			WHERE id = $1`

	// Declare a Movie struct to hold the data returned by the query
	var movie Movie

	// Use the context.WithTimeout() function to create a context.Context carries
	// a 3-seconds deadline
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)

	defer cancel()

	// Execute the query using the QueryRow method
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&movie.ID,
		&movie.CreatedAt,
		&movie.Title,
		&movie.Year,
		&movie.Runtime,
		pq.Array(&movie.Genres),
		&movie.Version,
	)

	// Handle any errors.
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	// Otherwise, return a pointer to the Movie struct
	return &movie, nil
}

// Update the whole record(even though you just need one filed)
func (m MovieModel) Update(movie *Movie) error {
	// Declare the SQL query for updating the whole record and returning the new version number
	query := `
			UPDATE movies
			SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
			WHERE id = $5 AND version = $6
			RETURNING version`

	// Create an args slice containing the values for the placeholder parameters
	args := []interface{}{
		movie.Title,
		movie.Year,
		movie.Runtime,
		pq.Array(movie.Genres),
		movie.ID,
		movie.Version, // For the data race
	}

	ctx, cancle := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancle()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

// 删除指定id的电影，并根据返回的影响行数来确定是否成功删除
func (m MovieModel) Delete(id int64) error {
	// Return an ErrRecordNotFound error if the movie ID is less than 1
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `DELETE FROM movies WHERE id = $1`

	ctx, cancle := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancle()

	// Execute the SQL query using the Exec method
	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	// Call the RowsAffected method on the sql.Result object to get the number of rows
	// affected by the query
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	// If no rows were affected, error
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// GetAll 根据用户的需求：标题，电影类型,以及所提供的过滤器（包含页面页码等信息），返回所有movies的列表（其中存放各个movie结构体的地址
func (m MovieModel) GetAll(title string, genres []string, filters Filters) ([]*Movie, Metadata, error) {
	query := fmt.Sprintf(`SELECT count(*) OVER(), id, created_at, title, year, runtime, genres, version
				FROM movies
				WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', $1) OR $1 = '')
				AND (genres @> $2 OR $2 = '{}')
				ORDER BY %s %s, id ASC
				LIMIT $3 OFFSET $4`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []interface{}{title, pq.Array(genres), filters.limit(), filters.offset()}

	// Use the QueryContext() to execute the query.This returns a sql.Rows resultset
	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}

	defer rows.Close()

	// 初始化一个总记录数
	// Initialize an empty slice to hold the movie data,全部存放的是地址
	totalRecords := 0
	movies := []*Movie{}

	for rows.Next() {
		var movie Movie

		err := rows.Scan(
			&totalRecords,
			&movie.ID,
			&movie.CreatedAt,
			&movie.Title,
			&movie.Year,
			&movie.Runtime,
			pq.Array(&movie.Genres),
			&movie.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}

		// Add the Movie struct to the slice.
		movies = append(movies, &movie)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	// 数据库操作完毕返回一个元数据结构体并最终返回
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	return movies, metadata, nil
}

// ValidateMovie 检验传来的movie对象是否能通过校验器中的检验方法
func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title", "must not be more than 500 bytes long")
	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int32(time.Now().Year()), "year", "must not be in the future")
	v.Check(movie.Runtime != 0, "runtime", "must be provided")
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres", "must not contain more than 5 genres")
	// Note that we're using the Unique helper in the line below to check that all
	// values in the movie.Genres slice are unique.
	v.Check(validator.Unique(movie.Genres), "genres", "must not contain duplicate values")
}
