-- tabela para a entidade - "Board[board]"
CREATE TABLE `BOARD` (
	ID BIGINT NOT NULL AUTO_INCREMENT,
	PRIMARY KEY(ID),
	`NAME` VARCHAR(255),
	`DESCRIPTION` VARCHAR(255),
	CREATION TIMESTAMP,
	MODIFICATION TIMESTAMP,
	USER_CREATION BIGINT NOT NULL,
	USER_MODIFICATION BIGINT,
	VERSION INTEGER NOT NULL
)
ENGINE=InnoDB 
DEFAULT CHARSET=utf8;


-- tabela intermedia entre Board e User
CREATE TABLE `USERS` (
	`BOARD` BIGINT NOT NULL
	,`USER` BIGINT NOT NULL
)
ENGINE=InnoDB 
DEFAULT CHARSET=utf8;

-- tabela para a entidade - "Lane[lane]"
CREATE TABLE `LANE` (
	ID BIGINT NOT NULL AUTO_INCREMENT,
	PRIMARY KEY(ID),
	`NAME` VARCHAR(255),
	`POSITION` BIGINT,
	`BOARD` BIGINT,
	CREATION TIMESTAMP,
	MODIFICATION TIMESTAMP,
	USER_CREATION BIGINT NOT NULL,
	USER_MODIFICATION BIGINT,
	VERSION INTEGER NOT NULL
)
ENGINE=InnoDB 
DEFAULT CHARSET=utf8;


-- tabela para a entidade - "User[user]"
CREATE TABLE `USER` (
	ID BIGINT NOT NULL AUTO_INCREMENT,
	PRIMARY KEY(ID),
	`NAME` VARCHAR(255),
	`USERNAME` VARCHAR(255),
	`PASSWORD` VARCHAR(255),
	`DEAD` BIGINT NOT NULL,
	CREATION TIMESTAMP,
	MODIFICATION TIMESTAMP,
	USER_CREATION BIGINT NOT NULL,
	USER_MODIFICATION BIGINT,
	VERSION INTEGER NOT NULL
)
ENGINE=InnoDB 
DEFAULT CHARSET=utf8;


-- tabela para a entidade - "Task[task]"
CREATE TABLE `TASK` (
	ID BIGINT NOT NULL AUTO_INCREMENT,
	PRIMARY KEY(ID),
	`TITLE` VARCHAR(255),
	`DETAIL` VARCHAR(255),
	`HEAD_COLOR` VARCHAR(255),
	`BODY_COLOR` VARCHAR(255),
	`POSITION` BIGINT,
	`USER` BIGINT,
	`LANE` BIGINT,
	CREATION TIMESTAMP,
	MODIFICATION TIMESTAMP,
	USER_CREATION BIGINT NOT NULL,
	USER_MODIFICATION BIGINT,
	VERSION INTEGER NOT NULL
)
ENGINE=InnoDB 
DEFAULT CHARSET=utf8;


-- tabela para a entidade - "Notification[notification]"
CREATE TABLE `NOTIFICATION` (
	ID BIGINT NOT NULL AUTO_INCREMENT,
	PRIMARY KEY(ID),
	`EMAIL` VARCHAR(255) NOT NULL,
	`TASK` BIGINT NOT NULL,
	`LANE` BIGINT NOT NULL,
	CREATION TIMESTAMP,
	MODIFICATION TIMESTAMP,
	USER_CREATION BIGINT NOT NULL,
	USER_MODIFICATION BIGINT,
	VERSION INTEGER NOT NULL
)
ENGINE=InnoDB 
DEFAULT CHARSET=utf8;


-- tabela para a entidade - "Role[role]"
CREATE TABLE `ROLE` (
	ID BIGINT NOT NULL AUTO_INCREMENT,
	PRIMARY KEY(ID),
	`KIND` VARCHAR(50) NOT NULL,
	`USER` BIGINT,
	CREATION TIMESTAMP,
	MODIFICATION TIMESTAMP,
	USER_CREATION BIGINT NOT NULL,
	USER_MODIFICATION BIGINT,
	VERSION INTEGER NOT NULL
)
ENGINE=InnoDB 
DEFAULT CHARSET=utf8;


-- CREATING ENTITY CONSTRAINTS "Board"
ALTER TABLE `BOARD` ADD CONSTRAINT UC_BOARD FOREIGN KEY (USER_CREATION) REFERENCES `USER` (ID);
ALTER TABLE `BOARD` ADD CONSTRAINT UM_BOARD FOREIGN KEY (USER_MODIFICATION) REFERENCES `USER` (ID);
ALTER TABLE `BOARD` ADD CONSTRAINT UK_BOARD1 UNIQUE (`NAME`);
ALTER TABLE `BOARD` ADD CONSTRAINT UK_BOARD2 UNIQUE (`DESCRIPTION`);
-- FKs da tabela intermedia BOARD + USER 
-- lado A (OWNER)
ALTER TABLE `USERS` ADD CONSTRAINT FK_USERS1 FOREIGN KEY (`BOARD`) REFERENCES `BOARD` (ID);
-- lado B
ALTER TABLE `USERS` ADD CONSTRAINT FK_USERS2 FOREIGN KEY (`USER`) REFERENCES `USER` (ID);
-- a combinacao lado A lado B eh unica
ALTER TABLE `USERS` ADD CONSTRAINT UK_USERS UNIQUE (`BOARD`, `USER`);
-- CREATING ENTITY CONSTRAINTS "Lane"
ALTER TABLE `LANE` ADD CONSTRAINT UC_LANE FOREIGN KEY (USER_CREATION) REFERENCES `USER` (ID);
ALTER TABLE `LANE` ADD CONSTRAINT UM_LANE FOREIGN KEY (USER_MODIFICATION) REFERENCES `USER` (ID);
ALTER TABLE `LANE` ADD CONSTRAINT UK_POS UNIQUE (`BOARD`, `POSITION`);
ALTER TABLE `LANE` ADD CONSTRAINT FK_LANE1 FOREIGN KEY (`BOARD`) REFERENCES `BOARD` (ID);
-- CREATING ENTITY CONSTRAINTS "User"
ALTER TABLE `USER` ADD CONSTRAINT UC_USER FOREIGN KEY (USER_CREATION) REFERENCES `USER` (ID);
ALTER TABLE `USER` ADD CONSTRAINT UM_USER FOREIGN KEY (USER_MODIFICATION) REFERENCES `USER` (ID);
ALTER TABLE `USER` ADD CONSTRAINT UK_ACTIVE_USER UNIQUE (`USERNAME`, `DEAD`);
-- CREATING ENTITY CONSTRAINTS "Task"
ALTER TABLE `TASK` ADD CONSTRAINT UC_TASK FOREIGN KEY (USER_CREATION) REFERENCES `USER` (ID);
ALTER TABLE `TASK` ADD CONSTRAINT UM_TASK FOREIGN KEY (USER_MODIFICATION) REFERENCES `USER` (ID);
ALTER TABLE `TASK` ADD CONSTRAINT UK_POS UNIQUE (`LANE`, `POSITION`);
ALTER TABLE `TASK` ADD CONSTRAINT FK_TASK1 FOREIGN KEY (`USER`) REFERENCES `USER` (ID);
ALTER TABLE `TASK` ADD CONSTRAINT FK_TASK2 FOREIGN KEY (`LANE`) REFERENCES `LANE` (ID);
-- CREATING ENTITY CONSTRAINTS "Notification"
ALTER TABLE `NOTIFICATION` ADD CONSTRAINT UC_NOTIFICATION FOREIGN KEY (USER_CREATION) REFERENCES `USER` (ID);
ALTER TABLE `NOTIFICATION` ADD CONSTRAINT UM_NOTIFICATION FOREIGN KEY (USER_MODIFICATION) REFERENCES `USER` (ID);
ALTER TABLE `NOTIFICATION` ADD CONSTRAINT FK_NOTIFICATION1 FOREIGN KEY (`TASK`) REFERENCES `TASK` (ID);
ALTER TABLE `NOTIFICATION` ADD CONSTRAINT FK_NOTIFICATION2 FOREIGN KEY (`LANE`) REFERENCES `LANE` (ID);
-- CREATING ENTITY CONSTRAINTS "Role"
ALTER TABLE `ROLE` ADD CONSTRAINT UC_ROLE FOREIGN KEY (USER_CREATION) REFERENCES `USER` (ID);
ALTER TABLE `ROLE` ADD CONSTRAINT UM_ROLE FOREIGN KEY (USER_MODIFICATION) REFERENCES `USER` (ID);
ALTER TABLE `ROLE` ADD CONSTRAINT UK_USER_KIND UNIQUE (`USER`, `KIND`);
ALTER TABLE `ROLE` ADD CONSTRAINT FK_ROLE1 FOREIGN KEY (`USER`) REFERENCES `USER` (ID);
