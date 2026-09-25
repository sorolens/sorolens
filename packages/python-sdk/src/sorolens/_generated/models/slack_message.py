from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

from ..models.slack_message_response_type import SlackMessageResponseType
from ..types import UNSET, Unset

if TYPE_CHECKING:
    from ..models.slack_message_blocks_item import SlackMessageBlocksItem


T = TypeVar("T", bound="SlackMessage")


@_attrs_define
class SlackMessage:
    """
    Attributes:
        response_type (SlackMessageResponseType):
        text (str):
        blocks (list[SlackMessageBlocksItem] | Unset):
    """

    response_type: SlackMessageResponseType
    text: str
    blocks: list[SlackMessageBlocksItem] | Unset = UNSET
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        response_type = self.response_type.value

        text = self.text

        blocks: list[dict[str, Any]] | Unset = UNSET
        if not isinstance(self.blocks, Unset):
            blocks = []
            for blocks_item_data in self.blocks:
                blocks_item = blocks_item_data.to_dict()
                blocks.append(blocks_item)

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "response_type": response_type,
                "text": text,
            }
        )
        if blocks is not UNSET:
            field_dict["blocks"] = blocks

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.slack_message_blocks_item import (
            SlackMessageBlocksItem,
        )

        d = dict(src_dict)
        response_type = SlackMessageResponseType(d.pop("response_type"))

        text = d.pop("text")

        _blocks = d.pop("blocks", UNSET)
        blocks: list[SlackMessageBlocksItem] | Unset = UNSET
        if _blocks is not UNSET:
            blocks = []
            for blocks_item_data in _blocks:
                blocks_item = SlackMessageBlocksItem.from_dict(blocks_item_data)

                blocks.append(blocks_item)

        slack_message = cls(
            response_type=response_type,
            text=text,
            blocks=blocks,
        )

        slack_message.additional_properties = d
        return slack_message

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
