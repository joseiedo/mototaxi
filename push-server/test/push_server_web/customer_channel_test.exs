defmodule PushServerWeb.CustomerChannelTest do
  use ExUnit.Case, async: true

  test "CustomerChannel module exists" do
    assert Code.ensure_loaded?(PushServerWeb.CustomerChannel)
  end

  test "join/3 is exported" do
    assert function_exported?(PushServerWeb.CustomerChannel, :join, 3)
  end
end
