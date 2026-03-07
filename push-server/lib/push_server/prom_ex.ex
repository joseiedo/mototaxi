defmodule PushServer.PromEx do
  use PromEx, otp_app: :push_server

  @impl true
  def plugins do
    [
      PromEx.Plugins.Beam,
      {PromEx.Plugins.Phoenix, router: PushServerWeb.Router, endpoint: PushServerWeb.Endpoint},
      PushServer.PromEx.CustomPlugin
    ]
  end
end

defmodule PushServer.PromEx.CustomPlugin do
  use PromEx.Plugin

  @connections_event [:push_server, :connections]
  @delivered_event   [:push_server, :messages, :delivered]
  @latency_event     [:push_server, :delivery, :latency]

  @impl true
  def event_metrics(_opts) do
    Event.build(:push_server_custom_metrics, [
      last_value(
        [:push_server, :connections, :active],
        event_name: @connections_event,
        measurement: :count,
        description: "Number of active push server WebSocket connections",
        tags: []
      ),
      sum(
        [:push_server, :messages, :delivered, :total],
        event_name: @delivered_event,
        measurement: :count,
        description: "Total number of location_update messages delivered to clients",
        tags: []
      ),
      distribution(
        [:push_server, :delivery, :latency, :milliseconds],
        event_name: @latency_event,
        measurement: :duration,
        description: "End-to-end delivery latency from emitted_at to push (ms)",
        reporter_options: [buckets: [10, 25, 50, 100, 250, 500, 1000, 2500]],
        tags: []
      )
    ])
  end
end
