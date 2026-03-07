defmodule PushServerWeb.UserSocketTest do
  use ExUnit.Case, async: true

  test "UserSocket module exists" do
    assert Code.ensure_loaded?(PushServerWeb.UserSocket)
  end

  test "CustomerChannel module exists" do
    assert Code.ensure_loaded?(PushServerWeb.CustomerChannel)
  end
end
