CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(200) NOT NULL,
    display_name VARCHAR(200),
    role VARCHAR(20) NOT NULL DEFAULT 'developer',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
