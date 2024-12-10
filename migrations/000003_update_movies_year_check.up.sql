-- Up migration: 修改约束
ALTER TABLE movies
DROP CONSTRAINT movies_year_check,
ADD CONSTRAINT movies_year_check CHECK (year >= 1888 AND year <= date_part('year', now()));
