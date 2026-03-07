defmodule PushServer.Application do
  use Application

  @impl true
  def start(_type, _args) do
    redis_url = Application.get_env(:push_server, :redis_url, "redis://redis:6379")

    children = [
      # PushServer.PromEx,            # Plan 04
      {Phoenix.PubSub, Application.get_env(:push_server, :pubsub, name: PushServer.PubSub)},
      {Redix, {redis_url, [name: :redix]}},
      # PushServer.Pipeline,          # Plan 03
      PushServerWeb.Endpoint
    ]

    opts = [strategy: :one_for_one, name: PushServer.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
