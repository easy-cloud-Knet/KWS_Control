-- ===========================================
-- KWS_Control Integration Test DB Init
-- ===========================================

-- ---- core_base database ----
CREATE DATABASE IF NOT EXISTS core_base;
USE core_base;

CREATE TABLE IF NOT EXISTS subnet (
    id INT PRIMARY KEY,
    last_subnet VARCHAR(64) NOT NULL
);
INSERT INTO subnet (id, last_subnet) VALUES (1, '10.0.0.1');

CREATE TABLE IF NOT EXISTS inst_info (
    uuid VARCHAR(128) PRIMARY KEY,
    inst_ip VARCHAR(64),
    guac_pass VARCHAR(256),
    inst_mem INT,
    inst_vcpu INT,
    inst_disk INT
);

CREATE TABLE IF NOT EXISTS inst_loc (
    uuid VARCHAR(128) PRIMARY KEY,
    core INT
);

-- ---- guacamole_db database ----
CREATE DATABASE IF NOT EXISTS guacamole_db;
USE guacamole_db;

CREATE TABLE IF NOT EXISTS guacamole_entity (
    entity_id     INT(11)      NOT NULL AUTO_INCREMENT,
    name          VARCHAR(128) NOT NULL,
    type          ENUM('USER') NOT NULL,
    PRIMARY KEY (entity_id),
    UNIQUE KEY UK_guacamole_entity_name_scope (type, name)
);

CREATE TABLE IF NOT EXISTS guacamole_user (
    user_id                 INT(11)      NOT NULL AUTO_INCREMENT,
    entity_id               INT(11)      NOT NULL,
    password_hash           BINARY(32)   NOT NULL,
    password_salt           BINARY(32),
    password_date           DATETIME     NOT NULL,
    full_name               VARCHAR(256),
    PRIMARY KEY (user_id),
    UNIQUE KEY UK_guacamole_user_single_entity (entity_id),
    CONSTRAINT FK_guacamole_user_entity
        FOREIGN KEY (entity_id)
        REFERENCES guacamole_entity (entity_id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS guacamole_connection (
    connection_id               INT(11)      NOT NULL AUTO_INCREMENT,
    connection_name             VARCHAR(128) NOT NULL,
    protocol                    VARCHAR(32)  NOT NULL,
    PRIMARY KEY (connection_id)
);

CREATE TABLE IF NOT EXISTS guacamole_connection_parameter (
    connection_id    INT(11)       NOT NULL,
    parameter_name   VARCHAR(128)  NOT NULL,
    parameter_value  VARCHAR(4096) NOT NULL,
    PRIMARY KEY (connection_id, parameter_name),
    CONSTRAINT FK_guacamole_connection_parameter_connection
        FOREIGN KEY (connection_id)
        REFERENCES guacamole_connection (connection_id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS guacamole_connection_permission (
    entity_id      INT(11) NOT NULL,
    connection_id  INT(11) NOT NULL,
    permission     ENUM('READ') NOT NULL,
    PRIMARY KEY (entity_id, connection_id, permission),
    CONSTRAINT FK_guacamole_connection_permission_entity
        FOREIGN KEY (entity_id)
        REFERENCES guacamole_entity (entity_id)
        ON DELETE CASCADE,
    CONSTRAINT FK_guacamole_connection_permission_connection
        FOREIGN KEY (connection_id)
        REFERENCES guacamole_connection (connection_id)
        ON DELETE CASCADE
);
