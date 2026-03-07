defmodule PushServer.PromExTest do
  use ExUnit.Case, async: true

  test "PromEx module exists" do
    assert Code.ensure_loaded?(PushServer.PromEx)
  end
end
