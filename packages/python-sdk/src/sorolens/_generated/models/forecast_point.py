from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset

from typing import cast
import datetime






T = TypeVar("T", bound="ForecastPoint")



@_attrs_define
class ForecastPoint:
    """ 
        Attributes:
            date (datetime.date):
            value (float):
            lower (float):
            upper (float):
     """

    date: datetime.date
    value: float
    lower: float
    upper: float
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        date = self.date.isoformat()

        value = self.value

        lower = self.lower

        upper = self.upper


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "date": date,
            "value": value,
            "lower": lower,
            "upper": upper,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        date = datetime.date.fromisoformat(d.pop("date"))




        value = d.pop("value")

        lower = d.pop("lower")

        upper = d.pop("upper")

        forecast_point = cls(
            date=date,
            value=value,
            lower=lower,
            upper=upper,
        )


        forecast_point.additional_properties = d
        return forecast_point

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
