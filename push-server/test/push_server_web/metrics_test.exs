defmodule PushServerWeb.MetricsTest do
  use ExUnit.Case, async: true

  test "Endpoint module is loadable" do
    assert Code.ensure_loaded?(PushServerWeb.Endpoint)
  end

  test "PromEx module is loadable" do
    assert Code.ensure_loaded?(PushServer.PromEx)
  end
end
