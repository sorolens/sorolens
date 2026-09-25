from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast

if TYPE_CHECKING:
  from ..models.event import Event





T = TypeVar("T", bound="ListContractEventsResponse200")



@_attrs_define
class ListContractEventsResponse200:
    """ 
        Attributes:
            events (list[Event]):
            next_cursor (str):
     """

    events: list[Event]
    next_cursor: str
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        from ..models.event import Event # noqa: PLC0415
        events = []
        for events_item_data in self.events:
            events_item = events_item_data.to_dict()
            events.append(events_item)



        next_cursor = self.next_cursor


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "events": events,
            "next_cursor": next_cursor,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        from ..models.event import Event # noqa: PLC0415
        d = dict(src_dict)
        events = []
        _events = d.pop("events")
        for events_item_data in (_events):
            events_item = Event.from_dict(events_item_data)



            events.append(events_item)


        next_cursor = d.pop("next_cursor")

        list_contract_events_response_200 = cls(
            events=events,
            next_cursor=next_cursor,
        )


        list_contract_events_response_200.additional_properties = d
        return list_contract_events_response_200

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
