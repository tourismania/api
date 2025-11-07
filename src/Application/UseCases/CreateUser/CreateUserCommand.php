<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\CreateUser;

use App\Domain\Entity\User;
use App\Domain\Services\UserCreator;
use App\Presentation\Http\Api\V1\CreateUser\CreateUserDto;

readonly class CreateUserCommand
{
    public function __construct(
        public string $firstName,
        public string $lastName,
        public string $email,
    )
    {
    }
}
