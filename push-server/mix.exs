defmodule PushServer.MixProject do
  use Mix.Project

  def project do
    [
      app: :push_server,
      version: "0.1.0",
      elixir: "~> 1.18",
      start_permanent: Mix.env() == :prod,
      deps: deps(),
      releases: [
        push_server: [include_executables_for: [:unix]]
      ]
    ]
  end

  def application do
    [
      extra_applications: [:logger, :runtime_tools],
      mod: {PushServer.Application, []}
    ]
  end

  defp deps do
    [
      {:phoenix, "~> 1.7"},
      {:phoenix_pubsub, "~> 2.1"},
      {:phoenix_pubsub_redis, "~> 3.0"},
      {:plug_cowboy, "~> 2.7"},
      {:broadway, "~> 1.2"},
      {:broadway_kafka, "~> 0.4"},
      {:redix, "~> 1.5"},
      {:jason, "~> 1.4"},
      {:prom_ex, "~> 1.11"},
      {:mox, "~> 1.0", only: :test}
    ]
  end
end
