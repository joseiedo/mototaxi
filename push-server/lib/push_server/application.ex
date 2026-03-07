defmodule PushServer.Application do
  use Application

  @impl true
  def start(_type, _args) do
    redis_url = Application.get_env(:push_server, :redis_url, "redis://redis:6379")
    pubsub_config = Application.get_env(:push_server, :pubsub, [
      name: PushServer.PubSub,
      adapter: Phoenix.PubSub.Redis,
      host: "redis",
      port: 6379,
      node_name: System.get_env("HOSTNAME", "push_server_dev")
    ])

    children = [
      # 1. PromEx first — captures startup telemetry
      PushServer.PromEx,
      # 2. PubSub with Redis adapter for cross-replica fan-out
      {Phoenix.PubSub, pubsub_config},
      # 3. Redix named connection for channel join Redis lookups
      {Redix, {redis_url, [name: :redix]}},
      # 4. Broadway pipeline — starts consuming Kafka
      PushServer.Pipeline,
      # 5. Endpoint last — starts accepting WebSocket connections
      PushServerWeb.Endpoint
    ]
    opts = [strategy: :one_for_one, name: PushServer.Supervisor]
    Supervisor.start_link(children, opts)
  end
end
