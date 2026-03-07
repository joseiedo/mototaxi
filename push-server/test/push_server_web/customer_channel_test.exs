defmodule PushServerWeb.CustomerChannelTest do
  use PushServerWeb.ChannelCase

  alias PushServer.RedixMock

  @driver_id "driver-1"
  @customer_id "customer-1"
  @location_payload %{
    "driver_id" => @driver_id,
    "lat" => -23.52,
    "lng" => -46.63,
    "bearing" => 142.5,
    "speed_kmh" => 34.2,
    "emitted_at" => "2026-03-06T17:40:02Z"
  }

  setup do
    {:ok, socket} = connect(PushServerWeb.UserSocket, %{})
    %{socket: socket}
  end

  # Case 1: Happy path — known customer, driver resolved, initial position present
  test "join/3 succeeds with known customer and pushes initial location_update", %{socket: socket} do
    expect(RedixMock, :command, fn :redix, ["GET", "customer:customer-1:driver"] ->
      {:ok, @driver_id}
    end)
    expect(RedixMock, :command, fn :redix, ["GET", "driver:driver-1:latest"] ->
      {:ok, Jason.encode!(@location_payload)}
    end)

    {:ok, _, _channel_socket} = subscribe_and_join(socket, "customer:#{@customer_id}", %{})

    assert_push "location_update", %{"driver_id" => "driver-1", "lat" => -23.52}
  end

  # Case 2: Unknown customer — Redis returns nil for customer key
  test "join/3 returns error unknown_customer when customer key absent", %{socket: socket} do
    expect(RedixMock, :command, fn :redix, ["GET", "customer:unknown-99:driver"] ->
      {:ok, nil}
    end)

    assert {:error, %{reason: "unknown_customer"}} =
      subscribe_and_join(socket, "customer:unknown-99", %{})
  end

  # Case 3: Redis unreachable — service_unavailable
  test "join/3 returns error service_unavailable when Redis errors", %{socket: socket} do
    expect(RedixMock, :command, fn :redix, ["GET", "customer:customer-1:driver"] ->
      {:error, %Redix.ConnectionError{reason: :closed}}
    end)

    assert {:error, %{reason: "service_unavailable"}} =
      subscribe_and_join(socket, "customer:#{@customer_id}", %{})
  end

  # Case 4: TTL expired — driver:*:latest is nil; join succeeds but no initial push
  test "join/3 succeeds and skips initial push when driver:*:latest TTL expired", %{socket: socket} do
    expect(RedixMock, :command, fn :redix, ["GET", "customer:customer-1:driver"] ->
      {:ok, @driver_id}
    end)
    expect(RedixMock, :command, fn :redix, ["GET", "driver:driver-1:latest"] ->
      {:ok, nil}
    end)

    {:ok, _, _channel_socket} = subscribe_and_join(socket, "customer:#{@customer_id}", %{})

    refute_push "location_update", _
  end

  # PubSub broadcast delivery
  test "handle_info/2 delivers Phoenix.Socket.Broadcast to client", %{socket: socket} do
    expect(RedixMock, :command, fn :redix, ["GET", "customer:customer-1:driver"] ->
      {:ok, @driver_id}
    end)
    expect(RedixMock, :command, fn :redix, ["GET", "driver:driver-1:latest"] ->
      {:ok, nil}
    end)

    {:ok, _, channel_socket} = subscribe_and_join(socket, "customer:#{@customer_id}", %{})

    broadcast = %Phoenix.Socket.Broadcast{
      topic: "driver:driver-1",
      event: "location_update",
      payload: @location_payload
    }
    send(channel_socket.channel_pid, broadcast)

    assert_push "location_update", %{"driver_id" => "driver-1"}
  end
end
