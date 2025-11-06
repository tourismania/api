<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\CreateUser;

use Symfony\Component\Serializer\Annotation\SerializedName;
use Symfony\Component\Validator\Constraints as Assert;

final readonly class CreateUserDto
{
    public function __construct(
        #[SerializedName('first_name')]
        #[Assert\NotBlank(message: 'Имя должно быть указано')]
        public string $firstName,

        #[SerializedName('last_name')]
        #[Assert\NotBlank(message: 'Фамилия должна быть указана')]
        public string $lastName,

        #[SerializedName('email')]
        #[Assert\NotBlank(message: 'Почта должна быть указана')]
        public string $email,
    ) {}
}
