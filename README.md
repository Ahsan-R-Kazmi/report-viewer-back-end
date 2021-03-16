# Segmed Demo Back-end

This is the back-end of the demo project. It uses Go, PostgreSQL, and Elasticsearch. The reports are saved in PostgreSQL to keep track of the tags and also in Elasticsearch to allow for efficient full-text search of the entirety of the text.

## Project Setup

### Go Setup
This project uses go version `go1.16.2`. The download page can be found here: https://golang.org/doc/install#download. The installation directions can be found here: https://golang.org/doc/install#install.

### Elasticsearch Setup
This project uses Elasticsearch version `7.11.2`. The download page and instructions can be found here: https://www.elastic.co/downloads/elasticsearch.

### PostgreSQL Setup
This project uses PostgreSQL version `13.0`. The download page and instructions can be found here:
https://www.postgresql.org/download/.

First, create the `segmed` database and then run the following scripts.

The following scripts must be run to create the tables required and insert some sample tag data. These scripts must be run prior to starting the app.
```
create table report
(
    id        bigserial not null
        constraint report_pk
            primary key,
    name      text      not null,
    author    text      not null,
    file_name text      not null,
    synopsis  text      not null,
    text      text      not null
);

alter table report
    owner to postgres;

create unique index report_id_uindex
    on report (id);

create unique index report_report_file_name_uindex
    on report (file_name);

create table tag
(
    id    bigserial not null
        constraint tag_pk
            primary key,
    name  text      not null,
    color text
);

alter table tag
    owner to postgres;

create unique index tag_id_uindex
    on tag (id);

create unique index tag_name_uindex
    on tag (name);

create table report_tag
(
    report_id bigint  not null
        constraint report_tag_report_id_fk
            references report,
    tag_id    bigint  not null
        constraint report_tag_tag_id_fk
            references tag,
    active    boolean not null
);

alter table report_tag
    owner to postgres;

create unique index report_tag_report_id_tag_id_uindex
    on report_tag (report_id, tag_id);

INSERT INTO tag(name, color) VALUES ('Fiction', '#000000');
INSERT INTO tag(name, color) VALUES ('Horror', '#FF0000');
INSERT INTO tag(name, color) VALUES ('Science Fiction', '#0000FF');
INSERT INTO tag(name, color) VALUES ('Adventure', '#00FF00');
INSERT INTO tag(name, color) VALUES ('Mystery', '#808080');
INSERT INTO tag(name, color) VALUES ('Non-fiction', '#FFFF00');
```
The database properties in the Go project are located in `segmed-demo-back-end/internal/db.go`.

## Starting the Application
1. Clone `segmed-demo-back-end` project into your `$HOME/go` directory:<br />
   `git clone https://github.com/Ahsan-R-Kazmi/segmed-demo-back-end.git`
2. After cloning completes, enter the project directory (i.e. `cd segmed-demo-back-end/`).
3. Initialize the project with a go module and then add any dependencies needed:<br />
   ```go mod init segmed-demo-back-end && go mod tidy```
4. The project can then be run:<br />
   `go run cmd/server.go`
   <br/>
   If you want to run it in the background use nohup e.g.:<br />
   `nohup go run cmd/server.go &`.
   <br/>
   (Conversely, you can use `go build` to create the binary and run that instead.)
   