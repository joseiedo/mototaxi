defmodule PushServerWeb.Endpoint do
  use Phoenix.Endpoint, otp_app: :push_server

  socket "/socket", PushServerWeb.UserSocket,
    websocket: [timeout: 45_000],
    longpoll: false

  plug PromEx.Plug, prom_ex_module: PushServer.PromEx
  plug PushServerWeb.Router
end
