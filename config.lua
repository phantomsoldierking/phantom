-- Phantom Developer Dashboard Configuration
-- This file is where you define your layout, commands, and custom panels.

--[[
    Hotkeys:
    - Tab / Shift+Tab: Cycle through panels
    - q or Ctrl+C: Quit
    - In HTTP Panel:
        - Enter: Send request when URL input is focused
        - Arrow Up/Down: Change focused input field
--]]

Config = {
    -- Define the layout of panels. Max 4 panels in a 2x2 grid for now.
    -- Available panel types: "workspace", "system", "http", and any custom panels you register.
    layout = {
        top_left = "workspace",
        top_right = "system",
        bottom_left = "http",
        bottom_right = "Clock" -- This is a custom panel defined below
    },

    -- Define custom shell commands that can be run from the workspace panel
    -- This feature is planned for a future update.
    commands = {
        { name = "test", command = "go test ./..." },
        { name = "run", command = "go run ." },
        { name = "build", command = "go build -o phantom" }
    },

    -- Pre-defined HTTP request templates for the HTTP panel
     http = {
        -- Environment variables can be used in requests with {{variable_name}}
        environment = {
            base_url = "https://jsonplaceholder.typicode.com",
            reqres_url = "https://reqres.in/api",
            auth_token = "Bearer your_jwt_token_here"
        },

        -- A collection of pre-defined request templates
        templates = {
            {
                name = "Get All Posts",
                method = "GET",
                url = "{{base_url}}/posts",
                headers = "",
                body = ""
            },
            {
                name = "Get Post #1",
                method = "GET",
                url = "{{base_url}}/posts/1",
                headers = "",
                body = ""
            },
            {
                name = "Create a Post",
                method = "POST",
                url = "{{base_url}}/posts",
                headers = 'Content-Type: application/json; charset=UTF-8',
                body = [[
{
    "title": "foo",
    "body": "bar",
    "userId": 1
}
                ]]
            },
            {
                name = "Update a Post (PUT)",
                method = "PUT",
                url = "{{base_url}}/posts/1",
                headers = 'Content-Type: application/json; charset=UTF-8',
                body = [[
{
    "id": 1,
    "title": "updated title",
    "body": "updated body",
    "userId": 1
}
                ]]
            },
            {
                name = "Get ReqRes User",
                method = "GET",
                url = "{{reqres_url}}/users/2",
                headers = "",
                body = ""
            },
            {
                name = "Protected Route Example",
                method = "GET",
                url = "https://api.example.com/me",
                headers = "Authorization: {{auth_token}}",
                body = ""
            }
        }
    }
}

-- 💡 CUSTOM PANELS 💡
-- Use phantom.register_panel(name, update_function) to create your own panels.
-- The function should return a string to be displayed.

-- 1. A simple clock panel
phantom.register_panel("Clock", function()
    -- os.date is a standard Lua function.
    return os.date("🕒 Time\n\n" .. os.date("%A, %B %d\n%Y %I:%M:%S %p"))
end)


-- 2. A panel showing Docker logs for a specific container
-- Note: Requires `docker` to be installed and the user in the `docker` group.
phantom.register_panel("Docker Logs", function()
    -- Replace 'my_container_name' with your actual container name or ID
    local container = "my_web_server"
    -- phantom.exec runs a shell command and returns its stdout.
    -- We're getting the last 10 lines of the logs.
    local handle = io.popen("docker logs --tail 10 " .. container .. " 2>&1")
    local result = handle:read("*a")
    handle:close()

    if result == "" or result == nil then
        return "🐳 Docker Logs: " .. container .. "\n\nContainer not running or not found."
    end
    
    return "🐳 Docker Logs: " .. container .. "\n\n" .. result
end)

-- Add another panel to the layout if you want to see it
-- For example, change a layout entry to "Docker Logs"
-- Config.layout.bottom_right = "Docker Logs"
