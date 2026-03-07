defmodule PushServer.Pipeline do
  use Broadway
  require Logger

  def start_link(_opts) do
    replicas = System.get_env("PUSH_SERVER_REPLICAS", "2") |> String.to_integer()
    multiplier = System.get_env("PARTITION_MULTIPLIER", "2") |> String.to_integer()
    concurrency = replicas * multiplier

    kafka_hosts = Application.get_env(:push_server, :kafka_hosts, [redpanda: 9092])

    Broadway.start_link(__MODULE__,
      name: __MODULE__,
      producer: [
        module: {BroadwayKafka.Producer, [
          hosts: kafka_hosts,
          group_id: "push_server",
          topics: ["driver.location"]
        ]},
        concurrency: 1
      ],
      processors: [
        default: [concurrency: concurrency]
      ]
    )
  end

  @impl true
  def handle_message(_processor, message, _context) do
    with {:ok, payload} <- Jason.decode(message.data),
         {:ok, driver_id} <- Map.fetch(payload, "driver_id") do
      Phoenix.PubSub.broadcast!(
        PushServer.PubSub,
        "driver:#{driver_id}",
        %Phoenix.Socket.Broadcast{
          topic: "driver:#{driver_id}",
          event: "location_update",
          payload: payload
        }
      )
    else
      _ -> Broadway.Message.failed(message, "decode_error")
    end

    message
  end

  @impl true
  def handle_failed(messages, _context) do
    Enum.each(messages, fn msg ->
      Logger.warning(
        "broadway handle_failed data=#{inspect(msg.data)} status=#{inspect(msg.status)}"
      )
    end)

    messages
  end
end
