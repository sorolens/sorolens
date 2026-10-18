from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, Self, TypeVar
from uuid import UUID

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="ContractNote")


@_attrs_define
class ContractNote:
    """
    Attributes:
        id (UUID):
        contract_id (str):
        author (str): Identity that wrote the note (X-User-ID). Only that identity may
            rewrite or delete it.
        body (str): Markdown source. Raw HTML is escaped rather than interpreted when
            the note is rendered.
        created_at (datetime.datetime):
        updated_at (datetime.datetime):
    """

    id: UUID
    contract_id: str
    author: str
    body: str
    created_at: datetime.datetime
    updated_at: datetime.datetime
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        id = str(self.id)

        contract_id = self.contract_id

        author = self.author

        body = self.body

        created_at = self.created_at.isoformat()

        updated_at = self.updated_at.isoformat()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "id": id,
                "contract_id": contract_id,
                "author": author,
                "body": body,
                "created_at": created_at,
                "updated_at": updated_at,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        id = UUID(d.pop("id"))

        contract_id = d.pop("contract_id")

        author = d.pop("author")

        body = d.pop("body")

        created_at = datetime.datetime.fromisoformat(d.pop("created_at"))

        updated_at = datetime.datetime.fromisoformat(d.pop("updated_at"))

        contract_note = cls(
            id=id,
            contract_id=contract_id,
            author=author,
            body=body,
            created_at=created_at,
            updated_at=updated_at,
        )

        contract_note.additional_properties = d
        return contract_note

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
