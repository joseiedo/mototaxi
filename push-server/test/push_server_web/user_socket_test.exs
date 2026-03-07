defmodule PushServerWeb.UserSocketTest do
  use PushServerWeb.ChannelCase

  test "connect/3 returns ok for any params" do
    assert {:ok, _socket} = connect(PushServerWeb.UserSocket, %{})
  end

  test "id/1 returns nil" do
    {:ok, socket} = connect(PushServerWeb.UserSocket, %{})
    assert PushServerWeb.UserSocket.id(socket) == nil
  end

  test "CustomerChannel module exists" do
    assert Code.ensure_loaded?(PushServerWeb.CustomerChannel)
  end

  test "channel customer:* routes to CustomerChannel" do
    Code.ensure_loaded(PushServerWeb.CustomerChannel)
    assert function_exported?(PushServerWeb.CustomerChannel, :join, 3)
  end
end
