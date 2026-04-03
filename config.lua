-- Phantom configuration
-- Loaded from:
--   1) ./config.lua
--   2) ~/.config/phantom/config.lua

Config = {
  logs = {
    -- File consumed by the Logs tab
    file = "debug.log",
    -- Multi-source merged logging (A1):
    -- type: "file" | "journald_unit" | "command"
    -- enabled defaults to true
    sources = {
      { name = "app",   type = "file",          path = "debug.log",                    color = "green",  enabled = true  },
      { name = "unit",  type = "journald_unit", unit = "ssh.service",                  color = "cyan",   enabled = false },
      { name = "sys",   type = "command",       cmd  = "journalctl -n 40 --no-pager",  color = "yellow", enabled = false },
    },
  },

  http = {
    -- Variables available as {{name}} in URL/headers/body.
    environment = {
      base_url = "https://jsonplaceholder.typicode.com",
      reqres_url = "https://reqres.in/api",
      auth_token = "Bearer your_jwt_token_here",
    },

    templates = {
      {
        name = "Get All Posts",
        method = "GET",
        url = "{{base_url}}/posts",
        headers = "",
        body = "",
      },
      {
        name = "Get Post #1",
        method = "GET",
        url = "{{base_url}}/posts/1",
        headers = "",
        body = "",
      },
      {
        name = "Create a Post",
        method = "POST",
        url = "{{base_url}}/posts",
        headers = "Content-Type: application/json",
        body = [[
{"title":"foo","body":"bar","userId":1}
]],
      },
      {
        name = "Update a Post (PUT)",
        method = "PUT",
        url = "{{base_url}}/posts/1",
        headers = "Content-Type: application/json",
        body = [[
{"id":1,"title":"updated title","body":"updated body","userId":1}
]],
      },
      {
        name = "Get ReqRes User",
        method = "GET",
        url = "{{reqres_url}}/users/2",
        headers = "",
        body = "",
      },
      {
        name = "Protected Route Example",
        method = "GET",
        url = "https://api.example.com/me",
        headers = "Authorization: {{auth_token}}",
        body = "",
      },
    },
  },
}
