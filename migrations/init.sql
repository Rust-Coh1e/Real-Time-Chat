CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY,
    username    TEXT NOT NULL UNIQUE,
    password_hash      TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rooms (
    id          UUID PRIMARY KEY,
    name    TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS msg (
    id          UUID PRIMARY KEY,
    room_id UUID,
    sender_id UUID,
    text    TEXT NOT NULL,
    created_at  TIMESTAMP DEFAULT NOW(),


    FOREIGN KEY (room_id) REFERENCES Rooms(id),
    FOREIGN KEY (sender_id) REFERENCES Users(id)

);

CREATE INDEX idx_msg_room_created ON msg(room_id, created_at);