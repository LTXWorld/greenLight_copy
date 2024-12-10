-- Down migration: 还原为原来的错误约束
ALTER TABLE movies
DROP CONSTRAINT movies_year_check,
ADD CONSTRAINT movies_year_check CHECK (year >= 18888 AND year::double precision <= date_part('year', now()));
