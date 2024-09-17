CREATE DATABASE requested_hands;

\c requested_hands;

CREATE TABLE poker_results (
    id SERIAL PRIMARY KEY,
    request_id VARCHAR(255),
    hand VARCHAR(255),
    result VARCHAR(255),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
