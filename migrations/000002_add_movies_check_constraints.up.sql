ALTER TABLE movies ADD CONSTRAINT movies_runtime_check CHECK ( runtime >= 0 );

ALTER TABLE movies ADD CONSTRAINT movies_year_check CHECK ( year BETWEEN 18888 AND date_part('year', now()) );

ALTER TABLE movies ADD CONSTRAINT genres_length_check CHECK ( array_length(genres, 1) BETWEEN 1 AND 5);