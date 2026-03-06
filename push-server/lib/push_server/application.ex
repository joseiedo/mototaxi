defmodule PushServer.Application do
  use Application

  @impl true
  def start(_type, _args) do
    children = [
      # PushServer.PromEx,            # Plan 04
      # {Phoenix.PubSub, Application.get_env(:push_server, :pubsub)},  # Plan 04
      # {Redix, {Application.get_env(:push_server, :redis_url, "redis://redis:6379"), name: :redix}},  # Plan 02
      # PushServer.Pipeline,          # Plan 03
      # PushServerWeb.Endpoint        # Plan 02
    ]
    opts = [strategy: :one_for_one, name: PushServer.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
