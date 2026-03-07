defmodule PushServerWeb.CustomerChannel do
  use Phoenix.Channel
  require Logger

  @redis_client Application.compile_env(:push_server, :redis_client, Redix)

  @impl true
  def join("customer:" <> customer_id, _params, socket) do
    case resolve_driver(customer_id) do
      {:ok, driver_id} ->
        PushServerWeb.Endpoint.subscribe("driver:#{driver_id}")
        socket =
          socket
          |> assign(:driver_id, driver_id)
          |> assign(:customer_id, customer_id)
        :telemetry.execute([:push_server, :connections], %{count: 1}, %{
          customer_id: customer_id
        })
        send(self(), {:push_initial, driver_id})
        {:ok, socket}

      {:error, reason} ->
        Logger.warning("join rejected customer_id=#{customer_id} reason=#{reason}")
        {:error, %{reason: reason}}
    end
  end

  @impl true
  def terminate(_reason, socket) do
    :telemetry.execute([:push_server, :connections], %{count: -1}, %{
      customer_id: socket.assigns[:customer_id]
    })
    :ok
  end

  @impl true
  def handle_info({:push_initial, driver_id}, socket) do
    case get_latest_position(driver_id) do
      {:ok, payload} ->
        push(socket, "location_update", payload)
        :telemetry.execute([:push_server, :messages, :delivered], %{count: 1}, %{})
        latency = compute_latency_ms(Map.get(payload, "emitted_at", ""))
        :telemetry.execute([:push_server, :delivery, :latency], %{duration: latency}, %{})
      {:skip} -> :ok
    end
    {:noreply, socket}
  end

  @impl true
  def handle_info(%Phoenix.Socket.Broadcast{event: event, payload: payload}, socket) do
    push(socket, event, payload)
    :telemetry.execute([:push_server, :messages, :delivered], %{count: 1}, %{})
    latency = compute_latency_ms(Map.get(payload, "emitted_at", ""))
    :telemetry.execute([:push_server, :delivery, :latency], %{duration: latency}, %{})
    {:noreply, socket}
  end

  defp resolve_driver(customer_id) do
    case @redis_client.command(:redix, ["GET", "customer:#{customer_id}:driver"]) do
      {:ok, nil} -> {:error, "unknown_customer"}
      {:ok, driver_id} -> {:ok, driver_id}
      {:error, _} -> {:error, "service_unavailable"}
    end
  end

  defp get_latest_position(driver_id) do
    case @redis_client.command(:redix, ["GET", "driver:#{driver_id}:latest"]) do
      {:ok, nil} -> {:skip}
      {:ok, json} -> {:ok, Jason.decode!(json)}
      {:error, _} -> {:skip}
    end
  end

  defp compute_latency_ms(emitted_at_str) do
    with {:ok, emitted_at, _} <- DateTime.from_iso8601(emitted_at_str) do
      DateTime.diff(DateTime.utc_now(), emitted_at, :millisecond)
    else
      _ -> 0
    end
  end
end
