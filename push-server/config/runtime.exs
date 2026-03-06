import Config

if config_env() == :prod do
  config :push_server, PushServerWeb.Endpoint,
    secret_key_base: System.fetch_env!("SECRET_KEY_BASE"),
    http: [port: String.to_integer(System.get_env("PORT", "4000"))]

  config :push_server, :pubsub,
    name: PushServer.PubSub,
    adapter: Phoenix.PubSub.Redis,
    host: System.get_env("REDIS_HOST", "redis"),
    port: String.to_integer(System.get_env("REDIS_PORT", "6379")),
    node_name: System.get_env("HOSTNAME", "push_server_default")

  config :push_server, :redis_url,
    "redis://#{System.get_env("REDIS_HOST", "redis")}:#{System.get_env("REDIS_PORT", "6379")}"

  config :push_server, :kafka_hosts,
    [{String.to_atom(System.get_env("KAFKA_HOST", "redpanda")),
      String.to_integer(System.get_env("KAFKA_PORT", "9092"))}]
end
