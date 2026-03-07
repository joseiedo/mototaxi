defmodule PushServerWeb.ChannelCase do
  use ExUnit.CaseTemplate

  using do
    quote do
      import Phoenix.ChannelTest
      @endpoint PushServerWeb.Endpoint
      import Mox
    end
  end

  setup do
    Mox.verify_on_exit!()
    :ok
  end
end
