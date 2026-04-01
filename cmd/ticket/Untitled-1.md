


tk init

tk init should be used to setup or review the current setup of a tk enabled project.

A tk-enabled project has a .ticket/config.json file at the root of the project (a sibling of a .git folder)

The `config.json` should contain

1. the "location" of the database: { "location": "<location-value>" }
    - local - a file path such as "filename.db", "../filename.db", "/path/to/file.db"
    OR
    - remote - an http/s address such as "http://localhost:8080" or "https://server.place.com"

2. the project ID this current path is associated with { "project_id": "<project-id>" }

---

for example

.ticket/config.json

remote:

{
    "location": "https://path/to/places"
    "project_id": "id",
}

or local:

{
    "location": "file:///path/to/the/ticket.db",
    "project_id": "id"
}

or local where the file is relative location to the location of the config.json

{
    "location": "ticket.db",
    "project_id": "id"
}


The .ticket folder *should* be a sibling of the .git folder.  I dont want to walk up to a. ~/.ticket location.  I'd rather walk up to a .git, then see ther is no .ticket folder tehre and tell the user that is where it shoudl be created.

There is no TICKET_URL but there can be a TICKET_USERNAME and TICKET_PASSWORD which would normally be used during calls.

during the tk init
IF already present - it should review the current file if any and explain the settings, prompting to change if required
IF remote
    if should prompt if the user wants to verify the connection (entering the username and password [via the ENV_VARS if present, *** for password)

It should ask - local or remote (1 or 2)

IF NOT present AND local:
    It should default local to be ".ticket/ticket.db"
    It should use the git remote of the project and hte folder name of the project as the project name

IF NOT present and remote:
    It shoudl accept a valid URL
    It shoud validate with a call to the URL using hte username/password supplied or available via TICKET_USERNAME/_PASSWORD
    It shoul send hte folder name and git remote origin during the check to see if there is a project
    The server should either return the project_id this is, or create a project for the user
    